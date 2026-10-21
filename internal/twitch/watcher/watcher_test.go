package watcher

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

type fakeGQL struct{ online bool }

func (f fakeGQL) Do(_ context.Context, op string, _ any, out any) error {
	if op != "PlaybackAccessToken" {
		return fmt.Errorf("unexpected op %s", op)
	}
	body := `{"streamPlaybackAccessToken":null}`
	if f.online {
		body = `{"streamPlaybackAccessToken":{"value":"{\"channel\":\"rivals\"}","signature":"sig"}}`
	}
	return json.Unmarshal([]byte(body), out)
}

// fakeTwitch serves usher, a rolling media playlist, segments, the channel page and spade.
type fakeTwitch struct {
	mu          sync.Mutex
	seq         int // first media sequence number in the playlist
	heads       map[string]int
	spade       []string
	expireAfter int // media playlist returns 403 after this many fetches (0 = never)
	mediaGets   int
	offline     bool
}

func (f *fakeTwitch) handler(t *testing.T) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /usher/rivals.m3u8", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("sig") != "sig" {
			t.Errorf("missing sig: %s", r.URL.RawQuery)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.offline {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, "#EXTM3U\n"+
			"#EXT-X-STREAM-INF:BANDWIDTH=6000000,RESOLUTION=1920x1080\nhigh/index.m3u8\n"+
			"#EXT-X-STREAM-INF:BANDWIDTH=230000,RESOLUTION=284x160\nlow/index.m3u8\n"+
			"#EXT-X-STREAM-INF:BANDWIDTH=1500000,RESOLUTION=852x480\nmid/index.m3u8\n")
	})
	mux.HandleFunc("GET /usher/low/index.m3u8", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.mediaGets++
		if f.expireAfter > 0 && f.mediaGets > f.expireAfter {
			f.mediaGets, f.expireAfter = 0, 0
			w.WriteHeader(http.StatusForbidden)
			return
		}
		fmt.Fprintf(w, "#EXTM3U\n#EXT-X-MEDIA-SEQUENCE:%d\n", f.seq)
		for i := f.seq; i < f.seq+3; i++ {
			fmt.Fprintf(w, "#EXTINF:2.000,live\nchunks/seg%d.ts\n", i)
		}
		f.seq++ // next fetch rolls the window by one segment
	})
	mux.HandleFunc("HEAD /usher/low/chunks/", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.heads[r.URL.Path]++
		f.mu.Unlock()
	})
	mux.HandleFunc("GET /web/rivals", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<script>window.__cfg={"beacon_url":"http://%s/spade"}</script>`, r.Host)
	})
	mux.HandleFunc("POST /spade", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.spade = append(f.spade, r.FormValue("data"))
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	return mux
}

func newWatcher(t *testing.T, f *fakeTwitch, online bool) (*Watcher, []Status, *sync.Mutex) {
	srv := httptest.NewServer(f.handler(t))
	t.Cleanup(srv.Close)
	var mu sync.Mutex
	var reports []Status
	w := &Watcher{
		GQL:           fakeGQL{online: online},
		UserID:        "42",
		UsherURL:      srv.URL + "/usher/",
		WebURL:        srv.URL + "/web/",
		PollInterval:  5 * time.Millisecond,
		SpadeInterval: time.Hour, // only the first poll sends spade
	}
	w.Report = func(s Status) { mu.Lock(); reports = append(reports, s); mu.Unlock() }
	return w, reports, &mu
}

var stream = Stream{
	Channel:  models.Channel{ID: "197886470", Login: "Rivals", GameID: "263490", BroadcastID: "555"},
	GameName: "Rust",
}

func TestWatchHeadsEachSegmentOnceAndSendsSpade(t *testing.T) {
	f := &fakeTwitch{heads: map[string]int{}}
	w, _, _ := newWatcher(t, f, true)
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := w.Watch(ctx, stream); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want deadline, got %v", err)
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.heads) < 5 {
		t.Fatalf("too few segments requested: %v", f.heads)
	}
	for path, n := range f.heads {
		if n != 1 {
			t.Errorf("segment %s requested %d times", path, n)
		}
		if !strings.HasPrefix(path, "/usher/low/") {
			t.Errorf("not the lowest variant: %s", path)
		}
	}
	if len(f.spade) != 1 {
		t.Fatalf("want 1 spade event, got %d", len(f.spade))
	}
	raw, err := base64.StdEncoding.DecodeString(f.spade[0])
	if err != nil {
		t.Fatal(err)
	}
	var events []struct {
		Event      string         `json:"event"`
		Properties map[string]any `json:"properties"`
	}
	if err := json.Unmarshal(raw, &events); err != nil {
		t.Fatal(err)
	}
	p := events[0].Properties
	if events[0].Event != "minute-watched" || p["channel_id"] != "197886470" || p["broadcast_id"] != "555" || p["user_id"] != float64(42) {
		t.Fatalf("bad spade event: %s", raw)
	}
}

func TestWatchRefreshesExpiredPlaylist(t *testing.T) {
	f := &fakeTwitch{heads: map[string]int{}, expireAfter: 2}
	w, _, _ := newWatcher(t, f, true)
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	w.Watch(ctx, stream)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.expireAfter != 0 {
		t.Fatal("playlist never expired")
	}
	if len(f.heads) < 5 {
		t.Fatalf("watching stopped after expiry: %d segments", len(f.heads))
	}
}

func TestWatchOffline(t *testing.T) {
	w, _, _ := newWatcher(t, &fakeTwitch{heads: map[string]int{}}, false)
	if err := w.Watch(context.Background(), stream); !errors.Is(err, ErrOffline) {
		t.Fatalf("no token: want ErrOffline, got %v", err)
	}
	w, _, _ = newWatcher(t, &fakeTwitch{heads: map[string]int{}, offline: true}, true)
	if err := w.Watch(context.Background(), stream); !errors.Is(err, ErrOffline) {
		t.Fatalf("usher 404: want ErrOffline, got %v", err)
	}
}

func TestLowestVariant(t *testing.T) {
	master := "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=900,CODECS=\"avc1\"\nb.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=160000,CODECS=\"mp4a.40.2\"\naudio.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=300\na.m3u8\n"
	if got := lowestVariant([]byte(master)); got != "a.m3u8" {
		t.Fatalf("got %q", got)
	}
}
