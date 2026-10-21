// Package facepunch reads the Rust drops schedule from twitch.facepunch.com.
//
// Twitch hides campaign listings from app clients behind an integrity check, so for
// Rust this public page is the source of which campaigns exist, their dates, the
// general drops and which streamers carry streamer-specific drops.
package facepunch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const URL = "https://twitch.facepunch.com/"

type Campaign struct {
	Round    string // e.g. "54"
	Title    string // e.g. "Base Invaders"
	Host     string // e.g. "Rustoria"
	Status   string // badge text: "Upcoming", "Live", ...
	StartAt  time.Time
	EndAt    time.Time
	General  []Drop // earned on any Rust stream with drops enabled
	Streamer []StreamerDrop
}

type Drop struct {
	ItemID   string
	Name     string
	Minutes  int
	ImageURL string
}

type StreamerDrop struct {
	Login       string // lower-case Twitch login taken from the channel link
	DisplayName string
	Drop        Drop
}

// Fetch downloads and parses the drops page.
func Fetch(ctx context.Context, httpClient *http.Client) ([]Campaign, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, URL, nil)
	if err != nil {
		return nil, err
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("facepunch: http %d", resp.StatusCode)
	}
	return Parse(resp.Body)
}

// Parse extracts campaigns from the page HTML. Campaign headers are
// "div.campaign.campaign-N"; general drop grids belong to the header before them,
// streamer sections name their campaign with a campaign-N class.
func Parse(r io.Reader) ([]Campaign, error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}
	byIndex := map[string]*Campaign{}
	var order []string
	current := ""

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch {
			case hasClass(n, "streamer-drops"):
				if c := byIndex[campaignIndex(n)]; c != nil {
					c.Streamer = append(c.Streamer, parseStreamerDrops(n)...)
				}
				return
			case hasClass(n, "campaign") && !hasClass(n, "section"):
				current = campaignIndex(n)
				if byIndex[current] == nil {
					byIndex[current] = &Campaign{}
					order = append(order, current)
				}
				parseHeader(n, byIndex[current])
			case hasClass(n, "general-drops-grid"):
				if c := byIndex[current]; c != nil {
					for _, box := range findAll(n, "drop-box") {
						c.General = append(c.General, parseDrop(box))
					}
				}
				return
			}
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(doc)

	out := make([]Campaign, 0, len(order))
	for _, idx := range order {
		out = append(out, *byIndex[idx])
	}
	return out, nil
}

func parseHeader(n *html.Node, c *Campaign) {
	if rounds := findAll(n, "event-round"); len(rounds) > 1 {
		c.Round = text(rounds[1])
	}
	if t := find(n, "event-title"); t != nil {
		c.Title = text(t)
	}
	if s := find(n, "event-subtitle"); s != nil {
		c.Host = strings.TrimSpace(strings.TrimPrefix(text(s), "Hosted by"))
	}
	if b := find(n, "status-badge"); b != nil {
		c.Status = text(b)
	}
	dates := findAll(n, "date")
	if len(dates) > 0 {
		c.StartAt = dateFromID(attr(dates[0], "data-date-id"))
	}
	if len(dates) > 1 {
		c.EndAt = dateFromID(attr(dates[1], "data-date-id"))
	}
}

func parseStreamerDrops(section *html.Node) []StreamerDrop {
	var out []StreamerDrop
	for _, box := range findAll(section, "drop-box") {
		sd := StreamerDrop{Drop: parseDrop(box)}
		if info := find(box, "streamer-info"); info != nil {
			sd.Login = loginFromURL(attr(info, "href"))
		}
		if name := find(box, "streamer-name"); name != nil {
			sd.DisplayName = text(name)
		}
		if sd.Login != "" {
			out = append(out, sd)
		}
	}
	return out
}

func parseDrop(box *html.Node) Drop {
	var d Drop
	if t := find(box, "drop-type"); t != nil {
		d.Name = text(t)
	}
	if t := find(box, "drop-time"); t != nil {
		d.Minutes = parseMinutes(text(t))
	}
	if c := find(box, "drop-counter"); c != nil {
		d.ItemID = attr(c, "data-itemid")
	}
	media := box
	if body := find(box, "drop-box-body"); body != nil {
		media = body // streamer boxes also hold the streamer avatar in the header
	}
	if img := findTag(media, "img"); img != nil {
		d.ImageURL = attr(img, "src")
	}
	return d
}

var (
	durationRe  = regexp.MustCompile(`(?i)(\d+)\s*(hour|hr|minute|min)`)
	campaignRe  = regexp.MustCompile(`^campaign-\d+$`)
	twitchURLRe = regexp.MustCompile(`(?i)twitch\.tv/([a-z0-9_]+)`)
)

// parseMinutes turns "1 Hour", "2 Hours", "30 minutes" or "1 Hour 30 Minutes" into minutes.
func parseMinutes(s string) int {
	total := 0
	for _, m := range durationRe.FindAllStringSubmatch(s, -1) {
		v, _ := strconv.Atoi(m[1])
		if strings.HasPrefix(strings.ToLower(m[2]), "h") {
			v *= 60
		}
		total += v
	}
	return total
}

// dateFromID reads the unix-seconds suffix of a data-date-id like "<hash>-1792778400".
func dateFromID(id string) time.Time {
	i := strings.LastIndexByte(id, '-')
	if i < 0 {
		return time.Time{}
	}
	sec, err := strconv.ParseInt(id[i+1:], 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(sec, 0).UTC()
}

func loginFromURL(u string) string {
	if m := twitchURLRe.FindStringSubmatch(u); m != nil {
		return strings.ToLower(m[1])
	}
	return ""
}

func campaignIndex(n *html.Node) string {
	for _, c := range strings.Fields(attr(n, "class")) {
		if campaignRe.MatchString(c) {
			return c
		}
	}
	return ""
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func hasClass(n *html.Node, class string) bool {
	for _, c := range strings.Fields(attr(n, "class")) {
		if c == class {
			return true
		}
	}
	return false
}

func findAll(n *html.Node, class string) []*html.Node {
	var out []*html.Node
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			if ch.Type == html.ElementNode && hasClass(ch, class) {
				out = append(out, ch)
				continue
			}
			walk(ch)
		}
	}
	walk(n)
	return out
}

func find(n *html.Node, class string) *html.Node {
	if all := findAll(n, class); len(all) > 0 {
		return all[0]
	}
	return nil
}

func findTag(n *html.Node, tag string) *html.Node {
	for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
		if ch.Type == html.ElementNode && ch.Data == tag {
			return ch
		}
		if f := findTag(ch, tag); f != nil {
			return f
		}
	}
	return nil
}

func text(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			b.WriteByte(' ')
		}
		for ch := n.FirstChild; ch != nil; ch = ch.NextSibling {
			walk(ch)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}
