// Package scheduler decides which channel to watch. It is pure: no I/O, no clock.
package scheduler

import (
	"slices"
	"strings"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

type Prefs struct {
	// Games in priority order (matched by name, case-insensitive). Empty means every game, equal priority.
	Games []string
	// ExcludedChannels by login, case-insensitive.
	ExcludedChannels []string
}

type Choice struct {
	Channel   models.Channel
	Campaigns []models.Campaign // campaigns progressed by watching Channel
}

// Pick returns the best channel to watch, or nil when nothing is worth watching.
//
// Ranking per channel uses its best campaign: game priority, then channel-restricted
// campaigns before open ones (short and exclusive), then the campaign ending first.
// Ties go to the channel covering more campaigns, then to more viewers.
func Pick(now time.Time, campaigns []models.Campaign, online []models.Channel, prefs Prefs) *Choice {
	eligible := make([]models.Campaign, 0, len(campaigns))
	for _, c := range campaigns {
		if c.Active(now) && c.AccountLinked && gameRank(prefs, c.Game.Name) >= 0 {
			eligible = append(eligible, c)
		}
	}

	var best *Choice
	var bestKey rank
	for _, ch := range online {
		if !ch.Online || !ch.DropsEnabled || excluded(prefs, ch.Login) {
			continue
		}
		var covered []models.Campaign
		for _, c := range eligible {
			if c.Allows(ch) {
				covered = append(covered, c)
			}
		}
		if len(covered) == 0 {
			continue
		}
		slices.SortFunc(covered, func(a, b models.Campaign) int { return campaignRank(prefs, a).cmp(campaignRank(prefs, b)) })
		key := campaignRank(prefs, covered[0])
		key.covered = -len(covered)
		key.viewers = -ch.Viewers
		if best == nil || key.cmp(bestKey) < 0 {
			best = &Choice{Channel: ch, Campaigns: covered}
			bestKey = key
		}
	}
	return best
}

// rank orders candidates; lower is better on every field.
type rank struct {
	game    int
	open    int // 0 restricted, 1 any channel
	end     time.Time
	covered int
	viewers int
}

func (r rank) cmp(o rank) int {
	switch {
	case r.game != o.game:
		return r.game - o.game
	case r.open != o.open:
		return r.open - o.open
	case !r.end.Equal(o.end):
		return r.end.Compare(o.end)
	case r.covered != o.covered:
		return r.covered - o.covered
	default:
		return r.viewers - o.viewers
	}
}

func campaignRank(prefs Prefs, c models.Campaign) rank {
	r := rank{game: gameRank(prefs, c.Game.Name), end: c.EndAt, open: 1}
	if c.Restricted() {
		r.open = 0
	}
	return r
}

// gameRank returns the priority index of game, or -1 if it is not wanted.
func gameRank(prefs Prefs, game string) int {
	if len(prefs.Games) == 0 {
		return 0
	}
	for i, g := range prefs.Games {
		if strings.EqualFold(g, game) {
			return i
		}
	}
	return -1
}

func excluded(prefs Prefs, login string) bool {
	for _, l := range prefs.ExcludedChannels {
		if strings.EqualFold(l, login) {
			return true
		}
	}
	return false
}
