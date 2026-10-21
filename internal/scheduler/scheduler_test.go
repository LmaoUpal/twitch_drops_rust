package scheduler

import (
	"testing"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

var (
	now  = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	rust = models.Game{ID: "263490", Name: "Rust"}
	cs2  = models.Game{ID: "32399", Name: "Counter-Strike"}

	rivals    = models.Channel{ID: "1", Login: "twitchrivals", Online: true, GameID: rust.ID, DropsEnabled: true, Viewers: 500}
	bigRust   = models.Channel{ID: "2", Login: "bigrust", Online: true, GameID: rust.ID, DropsEnabled: true, Viewers: 9000}
	smallRust = models.Channel{ID: "3", Login: "smallrust", Online: true, GameID: rust.ID, DropsEnabled: true, Viewers: 10}
	csChan    = models.Channel{ID: "4", Login: "cs", Online: true, GameID: cs2.ID, DropsEnabled: true, Viewers: 99999}
)

func campaign(id string, g models.Game, allowed []models.Channel, end time.Time) models.Campaign {
	return models.Campaign{
		ID: id, Name: id, Game: g, AccountLinked: true, AllowedChannels: allowed,
		StartAt: now.Add(-time.Hour), EndAt: end,
		Drops: []models.Drop{{ID: id + "-d", RequiredMinutes: 60, StartAt: now.Add(-time.Hour), EndAt: end}},
	}
}

func TestPick(t *testing.T) {
	end := now.Add(7 * 24 * time.Hour)
	general := campaign("rust-general", rust, nil, end)
	streamer := campaign("rust-rivals", rust, []models.Channel{rivals}, end)
	csCamp := campaign("cs", cs2, nil, now.Add(time.Hour))
	unlinked := streamer
	unlinked.AccountLinked = false
	claimed := streamer
	claimed.Drops = []models.Drop{{ID: "x", Claimed: true, StartAt: now.Add(-time.Hour), EndAt: end}}
	offlineRivals := rivals
	offlineRivals.Online = false

	rustFirst := Prefs{Games: []string{"rust", "Counter-Strike"}}

	tests := []struct {
		name      string
		campaigns []models.Campaign
		online    []models.Channel
		prefs     Prefs
		want      string // channel login, "" for nil
		covered   int
	}{
		{"streamer drop beats bigger general channel", []models.Campaign{general, streamer}, []models.Channel{bigRust, rivals}, rustFirst, "twitchrivals", 2},
		{"streamer offline falls back to most viewers", []models.Campaign{general, streamer}, []models.Channel{smallRust, offlineRivals, bigRust}, rustFirst, "bigrust", 1},
		{"game priority beats earlier end", []models.Campaign{csCamp, general}, []models.Channel{csChan, smallRust}, rustFirst, "smallrust", 1},
		{"unwanted game ignored", []models.Campaign{csCamp}, []models.Channel{csChan}, Prefs{Games: []string{"Rust"}}, "", 0},
		{"no prefs means any game", []models.Campaign{csCamp}, []models.Channel{csChan}, Prefs{}, "cs", 1},
		{"unlinked account skipped", []models.Campaign{unlinked}, []models.Channel{rivals}, rustFirst, "", 0},
		{"fully claimed campaign skipped", []models.Campaign{claimed, general}, []models.Channel{rivals, smallRust}, rustFirst, "twitchrivals", 1},
		{"excluded channel skipped", []models.Campaign{general, streamer}, []models.Channel{rivals, smallRust}, Prefs{Games: []string{"Rust"}, ExcludedChannels: []string{"TwitchRivals"}}, "smallrust", 1},
		{"drops disabled skipped", []models.Campaign{general}, []models.Channel{{ID: "9", Login: "nodrops", Online: true, GameID: rust.ID}}, rustFirst, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Pick(now, tt.campaigns, tt.online, tt.prefs)
			if tt.want == "" {
				if got != nil {
					t.Fatalf("want nil, got %s", got.Channel.Login)
				}
				return
			}
			if got == nil {
				t.Fatalf("want %s, got nil", tt.want)
			}
			if got.Channel.Login != tt.want || len(got.Campaigns) != tt.covered {
				t.Fatalf("want %s covering %d, got %s covering %d", tt.want, tt.covered, got.Channel.Login, len(got.Campaigns))
			}
		})
	}
}

func TestRankOrderReasonsAndMinutesLeft(t *testing.T) {
	end := now.Add(7 * 24 * time.Hour)
	general := campaign("rust-general", rust, nil, end)
	general.Drops = []models.Drop{
		{ID: "rug", RequiredMinutes: 60, CurrentMinutes: 60, StartAt: now.Add(-time.Hour), EndAt: end}, // done, unclaimed
		{ID: "bed", RequiredMinutes: 120, CurrentMinutes: 100, StartAt: now.Add(-time.Hour), EndAt: end},
	}
	streamer := campaign("rust-rivals", rust, []models.Channel{rivals}, end)
	got := Rank(now, []models.Campaign{general, streamer}, []models.Channel{smallRust, bigRust, rivals, bigRust}, Prefs{Games: []string{"Rust"}})
	if len(got) != 3 {
		t.Fatalf("want 3 unique channels, got %d", len(got))
	}
	if got[0].Channel.Login != "twitchrivals" || got[0].Reason != "streamer drop" {
		t.Fatalf("first: %s %s", got[0].Channel.Login, got[0].Reason)
	}
	if got[1].Channel.Login != "bigrust" || got[1].Reason != "general drops" || got[1].MinutesLeft != 20 {
		t.Fatalf("second: %+v", got[1])
	}
	if !Better(got[0], got[1]) || Better(got[1], got[2]) || Better(got[2], got[1]) {
		t.Fatal("Better: streamer beats general; general vs general never switches")
	}
}

func TestEligibleSkipsCompletedDrops(t *testing.T) {
	end := now.Add(time.Hour)
	c := campaign("done", rust, nil, end)
	c.Drops[0].CurrentMinutes = c.Drops[0].RequiredMinutes // complete but not claimed yet
	if len(Eligible(now, []models.Campaign{c}, Prefs{})) != 0 {
		t.Fatal("campaign whose drops are complete needs no more watching")
	}
}
