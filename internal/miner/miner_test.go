package miner

import (
	"testing"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
	"github.com/LmaoUpal/twitch_drops_rust/internal/scheduler"
)

var (
	now  = time.Date(2026, 10, 24, 12, 0, 0, 0, time.UTC)
	end  = now.Add(7 * 24 * time.Hour)
	rust = models.Game{ID: "263490", Name: "Rust", Slug: "rust"}

	rivals = models.Channel{ID: "1", Login: "twitchrivals", Online: true, GameID: rust.ID, DropsEnabled: true, Viewers: 500}
	bigA   = models.Channel{ID: "2", Login: "biga", Online: true, GameID: rust.ID, DropsEnabled: true, Viewers: 9000}
	bigB   = models.Channel{ID: "3", Login: "bigb", Online: true, GameID: rust.ID, DropsEnabled: true, Viewers: 8000}
	prefs  = scheduler.Prefs{Games: []string{"Rust"}}
)

func campaign(id string, allowed []models.Channel, drops ...models.Drop) models.Campaign {
	if len(drops) == 0 {
		drops = []models.Drop{{ID: id + "-d", Name: id, RequiredMinutes: 60, StartAt: now.Add(-time.Hour), EndAt: end}}
	}
	return models.Campaign{ID: id, Game: rust, AccountLinked: true, AllowedChannels: allowed,
		StartAt: now.Add(-time.Hour), EndAt: end, Drops: drops}
}

func TestDecide(t *testing.T) {
	general := campaign("general", nil)
	streamer := campaign("streamer", []models.Channel{rivals})
	offlineRivals := rivals
	offlineRivals.Online = false

	tests := []struct {
		name      string
		campaigns []models.Campaign
		live      []models.Channel
		current   *models.Channel
		action    Action
		target    string
		queue     int
	}{
		{"nothing watched picks best", []models.Campaign{general, streamer}, []models.Channel{bigA, bigB, rivals}, nil, Switch, "twitchrivals", 2},
		{"keeps a general channel despite bigger one", []models.Campaign{general}, []models.Channel{bigA, bigB}, &bigB, Keep, "bigb", 1},
		{"streamer drop interrupts general watch", []models.Campaign{general, streamer}, []models.Channel{bigA, rivals}, &bigA, Switch, "twitchrivals", 1},
		{"current channel no longer useful", []models.Campaign{streamer}, []models.Channel{rivals}, &bigA, Switch, "twitchrivals", 0},
		{"streamer offline falls back", []models.Campaign{general, streamer}, []models.Channel{bigA, offlineRivals}, &rivals, Switch, "biga", 0},
		{"idle when nothing eligible", []models.Campaign{streamer}, []models.Channel{bigA}, nil, Idle, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := Decide(now, tt.campaigns, tt.live, prefs, tt.current)
			if d.Action != tt.action {
				t.Fatalf("action %s, want %s", d.Action, tt.action)
			}
			if tt.target == "" {
				if d.Target != nil {
					t.Fatalf("want no target, got %s", d.Target.Channel.Login)
				}
				return
			}
			if d.Target.Channel.Login != tt.target || len(d.Queue) != tt.queue {
				t.Fatalf("target %s queue %d, want %s queue %d", d.Target.Channel.Login, len(d.Queue), tt.target, tt.queue)
			}
		})
	}
}

func TestGamesToScanAndUnexplored(t *testing.T) {
	streamerOnly := campaign("streamer", []models.Channel{rivals})
	if got := GamesToScan(now, []models.Campaign{streamerOnly}, prefs); len(got) != 0 {
		t.Fatalf("restricted campaigns need no directory scan: %v", got)
	}
	if got := GamesToScan(now, []models.Campaign{campaign("a", nil), campaign("b", nil)}, prefs); len(got) != 1 || got[0].Slug != "rust" {
		t.Fatalf("got %v", got)
	}
	var many []models.Channel
	for i := 0; i <= models.LookupLimit; i++ {
		many = append(many, models.Channel{ID: string(rune('a' + i)), Login: string(rune('a' + i))})
	}
	if got := GamesToScan(now, []models.Campaign{campaign("big", many)}, prefs); len(got) != 1 {
		t.Fatalf("long allow lists are found via the directory: %v", got)
	}
	got := Unexplored(now, []models.Campaign{campaign("a", nil)}, []string{"Rust", "Marvel Rivals"})
	if len(got) != 1 || got[0] != "Marvel Rivals" {
		t.Fatalf("got %v", got)
	}
}

func TestClaimable(t *testing.T) {
	c := campaign("c", nil,
		models.Drop{ID: "ready", RequiredMinutes: 30, CurrentMinutes: 30, InstanceID: "i1"},
		models.Drop{ID: "no-instance", RequiredMinutes: 30, CurrentMinutes: 30},
		models.Drop{ID: "claimed", RequiredMinutes: 30, CurrentMinutes: 30, InstanceID: "i2", Claimed: true},
		models.Drop{ID: "partial", RequiredMinutes: 30, CurrentMinutes: 10, InstanceID: "i3"},
	)
	got := Claimable([]models.Campaign{c})
	if len(got) != 1 || got[0].Drop.ID != "ready" {
		t.Fatalf("got %+v", got)
	}
}

func TestSlug(t *testing.T) {
	for name, want := range map[string]string{
		"Marvel Rivals":                  "marvel-rivals",
		"Tom Clancy's Rainbow Six Siege": "tom-clancys-rainbow-six-siege",
		"Counter-Strike":                 "counter-strike",
		"World of Tanks":                 "world-of-tanks",
	} {
		if got := Slug(models.Game{Name: name}); got != want {
			t.Errorf("Slug(%q) = %q, want %q", name, got, want)
		}
	}
	if Slug(models.Game{Name: "Rust", Slug: "rust-custom"}) != "rust-custom" {
		t.Error("explicit slug must win")
	}
}
