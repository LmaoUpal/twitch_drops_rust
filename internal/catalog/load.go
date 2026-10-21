package catalog

import (
	"context"
	"fmt"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/dropsapi"
	"github.com/LmaoUpal/twitch_drops_rust/internal/facepunch"
	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/gql"
)

// Twitch is the part of the GQL client Load needs.
type Twitch interface {
	Inventory(ctx context.Context) (gql.Inventory, error)
	StreamInfo(ctx context.Context, login string) (models.Channel, error)
}

// Sources are the campaign lists besides the Twitch inventory. Nil funcs are skipped.
type Sources struct {
	Facepunch func(context.Context) ([]facepunch.Campaign, error) // Rust schedule
	Catalog   func(context.Context) (dropsapi.Catalog, error)     // community catalog of running campaigns
	// Wanted filters catalog campaigns by game name; nil keeps every game.
	Wanted func(game string) bool
}

type Snapshot struct {
	Campaigns []models.Campaign    `json:"campaigns"`
	Claimed   []models.ClaimedDrop `json:"claimed"`
	UpdatedAt time.Time            `json:"updatedAt"`
	Warnings  []string             `json:"warnings"` // non-fatal source failures
}

// Load gathers campaigns from the Twitch inventory (authoritative, with progress), the
// community catalog and the Facepunch schedule, in that priority, then fills in live state
// for channel-restricted campaigns with short allow lists.
// Only an inventory failure is an error; other sources degrade to warnings.
func Load(ctx context.Context, tw Twitch, src Sources) (Snapshot, error) {
	snap := Snapshot{UpdatedAt: time.Now()}
	inv, err := tw.Inventory(ctx)
	if err != nil {
		return snap, fmt.Errorf("inventory: %w", err)
	}
	snap.Claimed = inv.Claimed
	wanted := src.Wanted
	if wanted == nil {
		wanted = func(string) bool { return true }
	}

	var fromFacepunch, fromCatalog []models.Campaign
	if src.Facepunch != nil {
		fp, err := src.Facepunch(ctx)
		if err != nil {
			snap.Warnings = append(snap.Warnings, "twitch.facepunch.com: "+err.Error())
		}
		fromFacepunch = FromFacepunch(fp)
	}
	if src.Catalog != nil {
		cat, err := src.Catalog(ctx)
		if err != nil {
			snap.Warnings = append(snap.Warnings, "drops catalog: "+err.Error())
		}
		fromCatalog = FromDropsAPI(cat, wanted)
	}
	snap.Campaigns = Merge(Merge(fromFacepunch, fromCatalog), inv.Campaigns)

	live := map[string]models.Channel{}
	for _, login := range RestrictedLogins(snap.Campaigns) {
		ch, err := tw.StreamInfo(ctx, login)
		if err != nil {
			snap.Warnings = append(snap.Warnings, "channel "+login+": "+err.Error())
			continue
		}
		live[login] = ch
	}
	ApplyChannels(snap.Campaigns, live)
	return snap, nil
}
