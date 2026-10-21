// Package activity keeps a bounded in-memory feed of app events for the Activity log screen.
// It is fed by a slog.Handler, so every slog call at Info or above shows up in the feed.
package activity

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Levels shown in the UI. "ok" marks successful milestones (log with slog attr ok=true).
const (
	LevelInfo  = "info"
	LevelOK    = "ok"
	LevelWarn  = "warn"
	LevelError = "error"
)

type Entry struct {
	Time    time.Time `json:"time"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
	Tag     string    `json:"tag"` // channel, game or component the event is about
}

// Feed is a ring buffer of entries with an optional listener.
type Feed struct {
	mu       sync.Mutex
	max      int
	entries  []Entry
	listener func(Entry)
}

func NewFeed(max int) *Feed { return &Feed{max: max} }

func (f *Feed) Add(e Entry) {
	f.mu.Lock()
	f.entries = append(f.entries, e)
	if len(f.entries) > f.max {
		f.entries = f.entries[len(f.entries)-f.max:]
	}
	l := f.listener
	f.mu.Unlock()
	if l != nil {
		l(e)
	}
}

// Entries returns a copy, oldest first.
func (f *Feed) Entries() []Entry {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Entry(nil), f.entries...)
}

// OnEntry sets the function called for every new entry (replaces the previous one).
func (f *Feed) OnEntry(fn func(Entry)) {
	f.mu.Lock()
	f.listener = fn
	f.mu.Unlock()
}

// Handler forwards records to Next and records Info+ ones in Feed.
type Handler struct {
	Next  slog.Handler
	Feed  *Feed
	attrs []slog.Attr
}

func (h *Handler) Enabled(ctx context.Context, l slog.Level) bool { return h.Next.Enabled(ctx, l) }

func (h *Handler) Handle(ctx context.Context, r slog.Record) error {
	if r.Level >= slog.LevelInfo {
		h.Feed.Add(h.entry(r))
	}
	return h.Next.Handle(ctx, r)
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &Handler{Next: h.Next.WithAttrs(attrs), Feed: h.Feed, attrs: append(append([]slog.Attr(nil), h.attrs...), attrs...)}
}

func (h *Handler) WithGroup(name string) slog.Handler {
	return &Handler{Next: h.Next.WithGroup(name), Feed: h.Feed, attrs: h.attrs}
}

// tagKeys are attrs that name what an event is about, in priority order.
var tagKeys = []string{"channel", "game", "tag"}

func (h *Handler) entry(r slog.Record) Entry {
	e := Entry{Time: r.Time, Message: r.Message, Level: LevelInfo}
	switch {
	case r.Level >= slog.LevelError:
		e.Level = LevelError
	case r.Level >= slog.LevelWarn:
		e.Level = LevelWarn
	}
	tags := map[string]string{}
	var details []string
	visit := func(a slog.Attr) bool {
		switch {
		case a.Key == "ok":
			if a.Value.Kind() == slog.KindBool && a.Value.Bool() && e.Level == LevelInfo {
				e.Level = LevelOK
			}
		case contains(tagKeys, a.Key):
			tags[a.Key] = a.Value.String()
		case a.Key == "err":
			details = append(details, a.Value.String())
		}
		return true
	}
	for _, a := range h.attrs {
		visit(a)
	}
	r.Attrs(visit)
	for _, k := range tagKeys {
		if tags[k] != "" {
			e.Tag = tags[k]
			break
		}
	}
	if len(details) > 0 {
		e.Message = fmt.Sprintf("%s: %s", e.Message, strings.Join(details, "; "))
	}
	return e
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
