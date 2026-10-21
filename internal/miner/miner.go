// Package miner holds the auto-mode decisions: which channel to watch next, when to switch
// and what can be claimed. It is pure like scheduler; the app runs the I/O around it.
package miner

import (
	"regexp"
	"strings"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
	"github.com/LmaoUpal/twitch_drops_rust/internal/scheduler"
)

type Action string

const (
	Keep   Action = "keep"   // the current channel is still the right one
	Switch Action = "switch" // watch Target instead
	Idle   Action = "idle"   // nothing worth watching
)

// QueueSize is how many runner-up channels a Decision lists for the UI.
const QueueSize = 5

type Decision struct {
	Action Action
	Target *scheduler.Choice // channel to watch (Keep: the current one)
	Queue  []scheduler.Choice
}

// Decide picks what to watch. current is the channel being watched, or nil.
// A running watch is only interrupted when it no longer progresses any eligible campaign,
// or when a clearly better option appears (see scheduler.Better).
func Decide(now time.Time, campaigns []models.Campaign, live []models.Channel, prefs scheduler.Prefs, current *models.Channel) Decision {
	ranked := scheduler.Rank(now, campaigns, live, prefs)
	var d Decision
	if current != nil {
		for i := range ranked {
			if strings.EqualFold(ranked[i].Channel.Login, current.Login) {
				d.Action, d.Target = Keep, &ranked[i]
				if !strings.EqualFold(ranked[0].Channel.Login, current.Login) && scheduler.Better(ranked[0], ranked[i]) {
					d.Action, d.Target = Switch, &ranked[0]
				}
				break
			}
		}
	}
	if d.Target == nil {
		if len(ranked) == 0 {
			return Decision{Action: Idle}
		}
		d.Action, d.Target = Switch, &ranked[0]
	}
	for _, c := range ranked {
		if len(d.Queue) == QueueSize {
			break
		}
		if !strings.EqualFold(c.Channel.Login, d.Target.Channel.Login) {
			d.Queue = append(d.Queue, c)
		}
	}
	return d
}

// GamesToScan returns games whose open (any-channel) campaigns need watching, so the app
// knows which game directories to list live drops streams from.
func GamesToScan(now time.Time, campaigns []models.Campaign, prefs scheduler.Prefs) []models.Game {
	seen := map[string]bool{}
	var out []models.Game
	for _, c := range scheduler.Eligible(now, campaigns, prefs) {
		if c.Restricted() || seen[c.Game.ID] {
			continue
		}
		seen[c.Game.ID] = true
		out = append(out, c.Game)
	}
	return out
}

// Unexplored returns wanted games with no known running or upcoming campaign. Twitch only
// reports campaigns once they have progress, so watching any drops stream of such a game
// for a while is the only way to find out whether it has drops.
func Unexplored(now time.Time, campaigns []models.Campaign, games []string) []string {
	var out []string
	for _, g := range games {
		known := false
		for _, c := range campaigns {
			if strings.EqualFold(c.Game.Name, g) && now.Before(c.EndAt) {
				known = true
				break
			}
		}
		if !known {
			out = append(out, g)
		}
	}
	return out
}

type ClaimItem struct {
	Campaign models.Campaign
	Drop     models.Drop
}

// Claimable returns drops that are complete and have a claim instance.
func Claimable(campaigns []models.Campaign) []ClaimItem {
	var out []ClaimItem
	for _, c := range campaigns {
		for _, d := range c.Drops {
			if d.Claimable() {
				out = append(out, ClaimItem{Campaign: c, Drop: d})
			}
		}
	}
	return out
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slug guesses a Twitch directory slug from a game name ("Marvel Rivals" -> "marvel-rivals").
func Slug(g models.Game) string {
	if g.Slug != "" {
		return g.Slug
	}
	s := strings.ToLower(g.Name)
	s = strings.NewReplacer("'", "", "’", "", "&", "and").Replace(s)
	return strings.Trim(nonSlug.ReplaceAllString(s, "-"), "-")
}
