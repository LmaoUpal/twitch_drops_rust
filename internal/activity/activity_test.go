package activity

import (
	"errors"
	"io"
	"log/slog"
	"testing"
)

func TestHandlerRecordsEntries(t *testing.T) {
	feed := NewFeed(3)
	var got []Entry
	feed.OnEntry(func(e Entry) { got = append(got, e) })
	log := slog.New(&Handler{Next: slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelDebug}), Feed: feed})

	log.Debug("not recorded")
	log.Info("Switched to TwitchRivals", "channel", "TwitchRivals", "ok", true)
	log.With("game", "Rust").Warn("Account not linked")
	log.Error("Inventory failed", "tag", "GQL", "err", errors.New("PersistedQueryNotFound"))
	log.Info("Campaigns refreshed")

	entries := feed.Entries()
	if len(entries) != 3 || len(got) != 4 {
		t.Fatalf("ring size 3: got %d entries, %d events", len(entries), len(got))
	}
	want := []Entry{
		{Level: LevelOK, Tag: "TwitchRivals", Message: "Switched to TwitchRivals"},
		{Level: LevelWarn, Tag: "Rust", Message: "Account not linked"},
		{Level: LevelError, Tag: "GQL", Message: "Inventory failed: PersistedQueryNotFound"},
		{Level: LevelInfo, Message: "Campaigns refreshed"},
	}
	for i, w := range want {
		g := got[i]
		if g.Level != w.Level || g.Tag != w.Tag || g.Message != w.Message || g.Time.IsZero() {
			t.Errorf("entry %d: got %+v, want %+v", i, g, w)
		}
	}
}
