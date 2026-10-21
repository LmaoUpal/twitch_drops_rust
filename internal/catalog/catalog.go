// Package catalog builds the list of known campaigns from every source the app has:
// the Twitch inventory (authoritative, with progress) and the Facepunch Rust schedule.
package catalog

import (
	"strings"

	"github.com/LmaoUpal/twitch_drops_rust/internal/facepunch"
	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

// Rust is the Twitch category Facepunch campaigns run in.
var Rust = models.Game{ID: "263490", Name: "Rust", Slug: "rust"}

const facepunchLinkURL = "https://twitch.facepunch.com/#account"

// FromFacepunch turns each Facepunch campaign into one open campaign for the general
// drops plus one channel-restricted campaign per streamer drop, as Twitch models them.
// Account linking is unknown here and assumed done.
func FromFacepunch(fp []facepunch.Campaign) []models.Campaign {
	var out []models.Campaign
	for _, c := range fp {
		name := "Rust Twitch Drops " + c.Round
		if c.Title != "" {
			name += ": " + c.Title
		}
		base := models.Campaign{
			Game: Rust, StartAt: c.StartAt, EndAt: c.EndAt,
			AccountLinked: true, LinkURL: facepunchLinkURL, Source: "facepunch",
		}
		if len(c.General) > 0 {
			general := base
			general.ID = "facepunch-" + c.Round + "-general"
			general.Name = name
			for _, d := range c.General {
				general.Drops = append(general.Drops, drop(d, c))
			}
			out = append(out, general)
		}
		for _, s := range c.Streamer {
			sc := base
			sc.ID = "facepunch-" + c.Round + "-" + s.Login
			sc.Name = name + " (" + s.DisplayName + ")"
			sc.AllowedChannels = []models.Channel{{Login: s.Login, DisplayName: s.DisplayName}}
			sc.Drops = []models.Drop{drop(s.Drop, c)}
			out = append(out, sc)
		}
	}
	return out
}

func drop(d facepunch.Drop, c facepunch.Campaign) models.Drop {
	return models.Drop{
		ID: "facepunch-" + d.ItemID, Name: d.Name, Benefits: []string{d.Name},
		RequiredMinutes: d.Minutes, ImageURL: d.ImageURL, StartAt: c.StartAt, EndAt: c.EndAt,
	}
}

// Merge returns the inventory campaigns followed by catalog campaigns the inventory does
// not cover yet. A catalog campaign is covered when an inventory campaign of the same game
// shares a drop name with it, with the same channel restriction.
func Merge(catalog, inventory []models.Campaign) []models.Campaign {
	out := append([]models.Campaign(nil), inventory...)
	for _, c := range catalog {
		covered := false
		for _, inv := range inventory {
			if covers(inv, c) {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, c)
		}
	}
	return out
}

func covers(inv, c models.Campaign) bool {
	if inv.Game.ID != c.Game.ID || inv.Restricted() != c.Restricted() {
		return false
	}
	if c.Restricted() && !sameChannels(inv.AllowedChannels, c.AllowedChannels) {
		return false
	}
	for _, a := range inv.Drops {
		for _, b := range c.Drops {
			if strings.EqualFold(strings.TrimSpace(a.Name), strings.TrimSpace(b.Name)) {
				return true
			}
		}
	}
	return false
}

// sameChannels reports whether every channel in want appears in have (by login).
func sameChannels(have, want []models.Channel) bool {
	for _, w := range want {
		found := false
		for _, h := range have {
			if strings.EqualFold(h.Login, w.Login) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// RestrictedLogins lists the distinct channel logins of all channel-restricted campaigns.
func RestrictedLogins(campaigns []models.Campaign) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range campaigns {
		for _, ch := range c.AllowedChannels {
			l := strings.ToLower(ch.Login)
			if l != "" && !seen[l] {
				seen[l] = true
				out = append(out, l)
			}
		}
	}
	return out
}

// ApplyChannels replaces allowed channels with fresh info (ID, live state, category),
// matched by login. Channels on a campaign's allow list are treated as drops-enabled.
func ApplyChannels(campaigns []models.Campaign, live map[string]models.Channel) {
	for i := range campaigns {
		for j, ch := range campaigns[i].AllowedChannels {
			if info, ok := live[strings.ToLower(ch.Login)]; ok {
				if info.DisplayName == "" {
					info.DisplayName = ch.DisplayName
				}
				info.DropsEnabled = true
				campaigns[i].AllowedChannels[j] = info
			}
		}
	}
}
