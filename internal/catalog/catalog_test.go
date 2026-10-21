package catalog

import (
	"testing"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/facepunch"
	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

var (
	start = time.Date(2026, 10, 23, 18, 0, 0, 0, time.UTC)
	end   = time.Date(2026, 11, 1, 23, 59, 0, 0, time.UTC)
	fp    = []facepunch.Campaign{{
		Round: "54", Title: "Base Invaders", StartAt: start, EndAt: end,
		General: []facepunch.Drop{{ItemID: "1", Name: "Rug", Minutes: 60}, {ItemID: "2", Name: "Bed", Minutes: 120}},
		Streamer: []facepunch.StreamerDrop{
			{Login: "twitchrivals", DisplayName: "TwitchRivals", Drop: facepunch.Drop{ItemID: "3", Name: "Backpack", Minutes: 30}},
		},
	}}
)

func TestFromFacepunch(t *testing.T) {
	got := FromFacepunch(fp)
	if len(got) != 2 {
		t.Fatalf("want general + 1 streamer campaign, got %d", len(got))
	}
	general, rivals := got[0], got[1]
	if general.Restricted() || general.Game.ID != Rust.ID || len(general.Drops) != 2 || general.Drops[1].RequiredMinutes != 120 {
		t.Errorf("general: %+v", general)
	}
	if !rivals.Restricted() || rivals.AllowedChannels[0].Login != "twitchrivals" || rivals.Drops[0].Name != "Backpack" {
		t.Errorf("rivals: %+v", rivals)
	}
	ch := models.Channel{ID: "197886470", Login: "TwitchRivals", GameID: Rust.ID}
	if !rivals.Allows(ch) || !general.Allows(ch) {
		t.Error("TwitchRivals playing Rust should progress both campaigns")
	}
}

func TestMerge(t *testing.T) {
	inventory := []models.Campaign{
		{ID: "tw-general", Game: Rust, Source: "twitch", Drops: []models.Drop{{Name: "rug", CurrentMinutes: 30}}},
		{ID: "tw-other", Game: models.Game{ID: "9"}, Source: "twitch", Drops: []models.Drop{{Name: "Hat"}}},
	}
	got := Merge(FromFacepunch(fp), inventory)
	var ids []string
	for _, c := range got {
		ids = append(ids, c.ID)
	}
	// general is covered by tw-general (shared drop "Rug"); the streamer campaign is not.
	want := []string{"tw-general", "tw-other", "facepunch-54-twitchrivals"}
	if len(ids) != len(want) {
		t.Fatalf("got %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("got %v, want %v", ids, want)
		}
	}
}

func TestMergeRestrictedNeedsSameChannel(t *testing.T) {
	inventory := []models.Campaign{{
		ID: "tw-other-streamer", Game: Rust, Source: "twitch",
		AllowedChannels: []models.Channel{{ID: "1", Login: "someoneelse"}},
		Drops:           []models.Drop{{Name: "Backpack"}},
	}}
	got := Merge(FromFacepunch(fp), inventory)
	if len(got) != 3 {
		t.Fatalf("another streamer's Backpack must not hide TwitchRivals', got %d campaigns", len(got))
	}
}

func TestApplyChannels(t *testing.T) {
	cs := FromFacepunch(fp)
	if logins := RestrictedLogins(cs); len(logins) != 1 || logins[0] != "twitchrivals" {
		t.Fatalf("logins %v", logins)
	}
	ApplyChannels(cs, map[string]models.Channel{
		"twitchrivals": {ID: "197886470", Login: "twitchrivals", Online: true, GameID: Rust.ID, Viewers: 100},
	})
	ch := cs[1].AllowedChannels[0]
	if ch.ID != "197886470" || !ch.Online || !ch.DropsEnabled || ch.DisplayName != "TwitchRivals" {
		t.Fatalf("got %+v", ch)
	}
}
