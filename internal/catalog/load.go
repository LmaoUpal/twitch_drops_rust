package catalog

import (
	"context"
	"fmt"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/facepunch"
	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/gql"
)

// Twitch is the part of the GQL client Load needs.
type Twitch interface {
	Inventory(ctx context.Context) (gql.Inventory, error)
	StreamInfo(ctx context.Context, login string) (models.Channel, error)
}

type Snapshot struct {
	Campaigns []models.Campaign    `json:"campaigns"`
	Claimed   []models.ClaimedDrop `json:"claimed"`
	UpdatedAt time.Time            `json:"updatedAt"`
	Warnings  []string             `json:"warnings"` // non-fatal source failures
}

// Load gathers campaigns from the Twitch inventory and, when withRust is set, the
// Facepunch schedule, then fills in live state for every channel-restricted campaign.
// Only an inventory failure is an error; other sources degrade to warnings.
func Load(ctx context.Context, tw Twitch, fetchFacepunch func(context.Context) ([]facepunch.Campaign, error), withRust bool) (Snapshot, error) {
	snap := Snapshot{UpdatedAt: time.Now()}
	inv, err := tw.Inventory(ctx)
	if err != nil {
		return snap, fmt.Errorf("inventory: %w", err)
	}
	inventory := inv.Campaigns
	snap.Claimed = inv.Claimed

	var fromFacepunch []models.Campaign
	if withRust {
		fp, err := fetchFacepunch(ctx)
		if err != nil {
			snap.Warnings = append(snap.Warnings, "twitch.facepunch.com: "+err.Error())
		}
		fromFacepunch = FromFacepunch(fp)
	}
	snap.Campaigns = Merge(fromFacepunch, inventory)

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
