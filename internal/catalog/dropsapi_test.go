package catalog

import (
	"testing"

	"github.com/LmaoUpal/twitch_drops_rust/internal/dropsapi"
	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

func catalogFixture() dropsapi.Catalog {
	return dropsapi.Catalog{Campaigns: []dropsapi.Campaign{
		{
			ID: "tw-rust", Name: "Rust Drops 54", Game: dropsapi.Game{ID: Rust.ID, Name: "Rust", Slug: "rust"},
			StartAt: start, EndAt: end,
			Drops: []dropsapi.Drop{
				{ID: "d-rug", Name: "Rug", RequiredMinutes: 60},
				{ID: "d-sub", Name: "Sub badge", RequiredSubs: 1}, // can't be earned by watching
			},
		},
		{
			ID: "tw-rivals", Name: "Backpack", Game: dropsapi.Game{ID: Rust.ID, Name: "Rust"},
			StartAt: start, EndAt: end,
			Channels: []dropsapi.Channel{{ID: "197886470", Login: "twitchrivals", DisplayName: "TwitchRivals"}},
			Drops:    []dropsapi.Drop{{ID: "d-backpack", Name: "Backpack", RequiredMinutes: 30}},
		},
		{
			ID: "tw-subonly", Name: "Subs only", Game: dropsapi.Game{ID: Rust.ID, Name: "Rust"},
			StartAt: start, EndAt: end,
			Drops: []dropsapi.Drop{{ID: "d-x", Name: "X", RequiredSubs: 1}},
		},
		{
			ID: "tw-other", Name: "Other", Game: dropsapi.Game{ID: "9", Name: "Warframe"},
			StartAt: start, EndAt: end,
			Drops: []dropsapi.Drop{{ID: "d-o", Name: "O", RequiredMinutes: 30}},
		},
	}}
}

func TestFromDropsAPI(t *testing.T) {
	got := FromDropsAPI(catalogFixture(), func(g string) bool { return g == "Rust" })
	if len(got) != 2 {
		t.Fatalf("want 2 Rust campaigns (sub-only and other game dropped), got %d", len(got))
	}
	general, rivals := got[0], got[1]
	if general.Source != "catalog" || general.Restricted() || len(general.Drops) != 1 || general.Drops[0].ID != "d-rug" || general.Game.Slug != "rust" {
		t.Errorf("general: %+v", general)
	}
	if !rivals.Restricted() || rivals.AllowedChannels[0].ID != "197886470" || !rivals.LookupChannels() {
		t.Errorf("rivals: %+v", rivals)
	}
	if !rivals.Allows(models.Channel{ID: "197886470", Login: "twitchrivals", GameID: Rust.ID}) {
		t.Error("catalog allow list matches by channel id")
	}
}

func TestMergePriority(t *testing.T) {
	cat := FromDropsAPI(catalogFixture(), func(g string) bool { return g == "Rust" })
	inventory := []models.Campaign{{ID: "tw-rust", Game: Rust, Source: "twitch", Drops: []models.Drop{{ID: "d-rug", Name: "Rug", CurrentMinutes: 20}}}}
	got := Merge(Merge(FromFacepunch(fp), cat), inventory)

	ids := map[string]string{}
	for _, c := range got {
		ids[c.ID] = c.Source
	}
	// Inventory wins over the catalog by id; the catalog hides both Facepunch entries
	// (general by the shared drop name "Rug", TwitchRivals by channel and "Backpack").
	if len(got) != 2 || ids["tw-rust"] != "twitch" || ids["tw-rivals"] != "catalog" {
		t.Fatalf("got %v", ids)
	}
}
