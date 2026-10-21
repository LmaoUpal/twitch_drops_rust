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
	Campaigns []models.Campaign // campaigns progressed by watching Channel, best first
	// MinutesLeft until the nearest drop on this channel completes (-1 if unknown).
	MinutesLeft int
	// Reason is a short label for the UI: "streamer drop", "general drops" or "ends soonest".
	Reason string

	key rank
}

// Pick returns the best channel to watch, or nil when nothing is worth watching.
func Pick(now time.Time, campaigns []models.Campaign, online []models.Channel, prefs Prefs) *Choice {
	ranked := Rank(now, campaigns, online, prefs)
	if len(ranked) == 0 {
		return nil
	}
	return &ranked[0]
}

// Rank returns every live channel that progresses at least one eligible campaign, best first.
//
// Ranking per channel uses its best campaign: game priority, then channel-restricted
// campaigns before open ones (short and exclusive), then the campaign ending first.
// Ties go to the channel covering more campaigns, then to more viewers.
func Rank(now time.Time, campaigns []models.Campaign, online []models.Channel, prefs Prefs) []Choice {
	eligible := Eligible(now, campaigns, prefs)
	var out []Choice
	seen := map[string]bool{}
	for _, ch := range online {
		login := strings.ToLower(ch.Login)
		if !ch.Online || !ch.DropsEnabled || excluded(prefs, ch.Login) || seen[login] {
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
		seen[login] = true
		slices.SortFunc(covered, func(a, b models.Campaign) int { return campaignRank(prefs, a).cmp(campaignRank(prefs, b)) })
		key := campaignRank(prefs, covered[0])
		key.covered = -len(covered)
		key.viewers = -ch.Viewers
		left := -1
		for _, c := range covered {
			if m := c.MinutesLeft(now); m >= 0 && (left < 0 || m < left) {
				left = m
			}
		}
		out = append(out, Choice{Channel: ch, Campaigns: covered, MinutesLeft: left, key: key})
	}
	slices.SortStableFunc(out, func(a, b Choice) int { return a.key.cmp(b.key) })
	for i := range out {
		switch {
		case out[i].key.open == 0:
			out[i].Reason = "streamer drop"
		case i > 0 && out[i].key.end.Before(out[0].key.end):
			out[i].Reason = "ends soonest"
		default:
			out[i].Reason = "general drops"
		}
	}
	return out
}

// Eligible returns campaigns worth watching: running, with drops still needing watch
// time, account linked, and for a wanted game.
func Eligible(now time.Time, campaigns []models.Campaign, prefs Prefs) []models.Campaign {
	var out []models.Campaign
	for _, c := range campaigns {
		if c.NeedsWatch(now) && c.AccountLinked && gameRank(prefs, c.Game.Name) >= 0 {
			out = append(out, c)
		}
	}
	return out
}

// Better reports whether a is worth switching to from b: a wanted game ranked higher,
// or a channel-restricted campaign against an open one of the same game priority.
// Smaller differences (viewers, end dates) never justify interrupting a watch.
func Better(a, b Choice) bool {
	if a.key.game != b.key.game {
		return a.key.game < b.key.game
	}
	return a.key.open < b.key.open
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
