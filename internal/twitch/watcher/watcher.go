// Package watcher keeps a live channel "watched" without decoding video.
//
// Since 2026-10-07 Twitch only counts watch time while HLS media segments are being
// requested, so the watcher polls the lowest-bandwidth media playlist and sends a HEAD
// request for every new segment (no audio/video is downloaded). A minute-watched spade
// event is still sent once a minute as the web player does.
package watcher

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

const (
	DefaultPollInterval  = 10 * time.Second
	DefaultSpadeInterval = 60 * time.Second
	fallbackSpadeURL     = "https://spade.twitch.tv/track"
	usherURL             = "https://usher.ttvnw.net/api/channel/hls/"
	webURL               = "https://www.twitch.tv/"
	segmentCacheSize     = 256
)

// ErrOffline means the channel stopped streaming.
var ErrOffline = errors.New("watcher: channel offline")

// GQL is the subset of the GQL client the watcher uses.
type GQL interface {
	Do(ctx context.Context, op string, vars, out any) error
}

// Stream identifies what to watch; Channel needs ID, Login and BroadcastID.
type Stream struct {
	Channel  models.Channel
	GameName string
}

// Status is reported after every poll.
type Status struct {
	Segments    int       `json:"segments"`    // segments requested successfully so far
	SpadeEvents int       `json:"spadeEvents"` // minute-watched events accepted so far
	LastPoll    time.Time `json:"lastPoll"`
	Err         string    `json:"error"` // last transient error, if any
}

type Watcher struct {
	GQL       GQL
	HTTP      *http.Client
	UserID    string
	UserAgent string
	// UsherURL, WebURL and SpadeURL override Twitch endpoints in tests.
	UsherURL string
	WebURL   string
	SpadeURL string

	PollInterval  time.Duration
	SpadeInterval time.Duration
	Report        func(Status)
	Log           *slog.Logger

	spadeMu    sync.Mutex
	spadeFound string // cached endpoint discovered from the channel page
}

// Watch blocks until ctx is cancelled (returns ctx.Err()) or the stream ends (ErrOffline).
func (w *Watcher) Watch(ctx context.Context, s Stream) error {
	log := w.logger().With("channel", s.Channel.Login)
	variant, err := w.variantURL(ctx, s.Channel.Login)
	if err != nil {
		return err
	}
	spadeURL := w.spadeURL(ctx, s.Channel.Login)
	payload, err := spadePayload(s, w.UserID)
	if err != nil {
		return err
	}

	seen := newLRU(segmentCacheSize)
	var st Status
	poll := time.NewTicker(w.pollInterval())
	defer poll.Stop()
	var lastSpade time.Time

	for {
		st.LastPoll = time.Now()
		st.Err = ""
		n, err := w.pollSegments(ctx, variant, seen)
		st.Segments += n
		switch {
		case errors.Is(err, errPlaylistExpired):
			log.Debug("media playlist expired, refreshing")
			variant, err = w.variantURL(ctx, s.Channel.Login)
			if err != nil {
				return err
			}
		case err != nil && ctx.Err() == nil:
			st.Err = err.Error()
			log.Warn("Segment requests failed", "err", err)
		}

		if time.Since(lastSpade) >= w.spadeInterval() {
			if err := w.sendSpade(ctx, spadeURL, payload); err != nil {
				if ctx.Err() == nil {
					st.Err = err.Error()
					log.Warn("minute-watched event failed", "err", err)
				}
			} else {
				st.SpadeEvents++
				lastSpade = time.Now()
			}
		}
		if w.Report != nil {
			w.Report(st)
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-poll.C:
		}
	}
}

var errPlaylistExpired = errors.New("watcher: media playlist expired")

// variantURL gets a playback token and returns the lowest-bandwidth media playlist URL.
func (w *Watcher) variantURL(ctx context.Context, login string) (string, error) {
	var tok struct {
		StreamPlaybackAccessToken *struct {
			Value     string `json:"value"`
			Signature string `json:"signature"`
		} `json:"streamPlaybackAccessToken"`
	}
	vars := map[string]any{"isLive": true, "isVod": false, "login": login, "platform": "web", "playerType": "site", "vodID": ""}
	if err := w.GQL.Do(ctx, "PlaybackAccessToken", vars, &tok); err != nil {
		return "", fmt.Errorf("playback token: %w", err)
	}
	if tok.StreamPlaybackAccessToken == nil {
		return "", ErrOffline
	}
	q := url.Values{
		"sig":              {tok.StreamPlaybackAccessToken.Signature},
		"token":            {tok.StreamPlaybackAccessToken.Value},
		"allow_source":     {"true"},
		"allow_audio_only": {"true"},
		"p":                {strconv.Itoa(int(time.Now().UnixNano() % 1_000_000))},
	}
	base := w.UsherURL
	if base == "" {
		base = usherURL
	}
	master := base + url.PathEscape(strings.ToLower(login)) + ".m3u8?" + q.Encode()
	status, body, err := w.get(ctx, master)
	if err != nil {
		return "", fmt.Errorf("master playlist: %w", err)
	}
	if status == http.StatusNotFound {
		return "", ErrOffline
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("master playlist: http %d", status)
	}
	variant := lowestVariant(body)
	if variant == "" {
		return "", errors.New("master playlist: no variants")
	}
	return resolve(master, variant), nil
}

// pollSegments fetches the media playlist and HEADs every segment not seen before.
func (w *Watcher) pollSegments(ctx context.Context, variant string, seen *lru) (int, error) {
	status, body, err := w.get(ctx, variant)
	if err != nil {
		return 0, err
	}
	if status == http.StatusForbidden || status == http.StatusNotFound {
		return 0, errPlaylistExpired
	}
	if status != http.StatusOK {
		return 0, fmt.Errorf("media playlist: http %d", status)
	}
	ok := 0
	var lastErr error
	for _, seg := range segments(body) {
		u := resolve(variant, seg)
		if seen.has(u) {
			continue
		}
		hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		status, err := w.head(hctx, u)
		cancel()
		if err != nil || status >= 400 {
			if err == nil {
				err = fmt.Errorf("segment: http %d", status)
			}
			lastErr = err
			continue // retried on the next poll
		}
		seen.add(u)
		ok++
	}
	return ok, lastErr
}

var (
	beaconRe   = regexp.MustCompile(`"beacon_?url": ?"(https?://[^"]+)"`)
	settingsRe = regexp.MustCompile(`https://[a-z0-9.]+/config/settings\.[0-9a-f]{32}\.js`)
)

// spadeURL finds the minute-watched endpoint in the channel page or its settings script.
func (w *Watcher) spadeURL(ctx context.Context, login string) string {
	if w.SpadeURL != "" {
		return w.SpadeURL
	}
	w.spadeMu.Lock()
	defer w.spadeMu.Unlock()
	if w.spadeFound != "" {
		return w.spadeFound
	}
	base := w.WebURL
	if base == "" {
		base = webURL
	}
	_, page, err := w.get(ctx, base+strings.ToLower(login))
	if err == nil {
		if m := beaconRe.FindSubmatch(page); m != nil {
			w.spadeFound = string(m[1])
			return w.spadeFound
		}
		if s := settingsRe.Find(page); s != nil {
			if _, js, err := w.get(ctx, string(s)); err == nil {
				if m := beaconRe.FindSubmatch(js); m != nil {
					w.spadeFound = string(m[1])
					return w.spadeFound
				}
			}
		}
	}
	w.logger().Debug("spade url not found, using fallback", "err", err)
	return fallbackSpadeURL
}

func spadePayload(s Stream, userID string) (string, error) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return "", fmt.Errorf("watcher: user id %q: %w", userID, err)
	}
	event := []map[string]any{{
		"event": "minute-watched",
		"properties": map[string]any{
			"broadcast_id":   s.Channel.BroadcastID,
			"channel_id":     s.Channel.ID,
			"channel":        s.Channel.Login,
			"game":           s.GameName,
			"game_id":        s.Channel.GameID,
			"hidden":         false,
			"is_live":        true,
			"live":           true,
			"location":       "channel",
			"logged_in":      true,
			"muted":          false,
			"minutes_logged": 1,
			"player":         "site",
			"user_id":        uid,
		},
	}}
	raw, err := json.Marshal(event)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func (w *Watcher) sendSpade(ctx context.Context, spadeURL, payload string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, spadeURL, strings.NewReader(url.Values{"data": {payload}}.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := w.do(req)
	if err != nil {
		return err
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("spade: http %d", resp.StatusCode)
	}
	return nil
}

// lowestVariant picks the variant with the smallest BANDWIDTH from a master playlist.
func lowestVariant(master []byte) string {
	best, bestBW := "", -1
	bw := -1
	sc := bufio.NewScanner(strings.NewReader(string(master)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "#EXT-X-STREAM-INF:"):
			bw = attrInt(line, "BANDWIDTH")
		case line != "" && !strings.HasPrefix(line, "#") && bw >= 0:
			if bestBW < 0 || bw < bestBW {
				best, bestBW = line, bw
			}
			bw = -1
		}
	}
	return best
}

// segments returns media segment URIs (lines following #EXTINF) of a media playlist.
func segments(media []byte) []string {
	var out []string
	next := false
	sc := bufio.NewScanner(strings.NewReader(string(media)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		switch {
		case strings.HasPrefix(line, "#EXTINF:"):
			next = true
		case next && line != "" && !strings.HasPrefix(line, "#"):
			out = append(out, line)
			next = false
		}
	}
	return out
}

func attrInt(line, key string) int {
	i := strings.Index(line, key+"=")
	if i < 0 {
		return 0
	}
	v := line[i+len(key)+1:]
	if j := strings.IndexByte(v, ','); j >= 0 {
		v = v[:j]
	}
	n, _ := strconv.Atoi(v)
	return n
}

func resolve(base, ref string) string {
	b, err := url.Parse(base)
	if err != nil {
		return ref
	}
	r, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return b.ResolveReference(r).String()
}

func (w *Watcher) get(ctx context.Context, u string) (int, []byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := w.do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return resp.StatusCode, body, err
}

func (w *Watcher) head(ctx context.Context, u string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, u, nil)
	if err != nil {
		return 0, err
	}
	resp, err := w.do(req)
	if err != nil {
		return 0, err
	}
	resp.Body.Close()
	return resp.StatusCode, nil
}

func (w *Watcher) do(req *http.Request) (*http.Response, error) {
	if w.UserAgent != "" {
		req.Header.Set("User-Agent", w.UserAgent)
	}
	c := w.HTTP
	if c == nil {
		c = http.DefaultClient
	}
	return c.Do(req)
}

func (w *Watcher) pollInterval() time.Duration {
	if w.PollInterval > 0 {
		return w.PollInterval
	}
	return DefaultPollInterval
}

func (w *Watcher) spadeInterval() time.Duration {
	if w.SpadeInterval > 0 {
		return w.SpadeInterval
	}
	return DefaultSpadeInterval
}

func (w *Watcher) logger() *slog.Logger {
	if w.Log != nil {
		return w.Log
	}
	return slog.Default()
}

// lru is a bounded set of recently requested segment URLs.
type lru struct {
	max   int
	order []string
	set   map[string]bool
}

func newLRU(max int) *lru { return &lru{max: max, set: map[string]bool{}} }

func (l *lru) has(k string) bool { return l.set[k] }

func (l *lru) add(k string) {
	l.set[k] = true
	l.order = append(l.order, k)
	if len(l.order) > l.max {
		delete(l.set, l.order[0])
		l.order = l.order[1:]
	}
}
