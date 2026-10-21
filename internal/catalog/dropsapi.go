package catalog

import (
	"github.com/LmaoUpal/twitch_drops_rust/internal/dropsapi"
	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

// FromDropsAPI converts community-catalog campaigns of wanted games. Drops that need
// subscriptions instead of watch time are left out, and so are campaigns with none left.
// Account linking is unknown here and assumed done until the inventory says otherwise.
func FromDropsAPI(cat dropsapi.Catalog, wanted func(game string) bool) []models.Campaign {
	var out []models.Campaign
	for _, c := range cat.Campaigns {
		if !wanted(c.Game.Name) {
			continue
		}
		mc := models.Campaign{
			ID: c.ID, Name: c.Name,
			Game:    models.Game{ID: c.Game.ID, Name: c.Game.Name, Slug: c.Game.Slug},
			StartAt: c.StartAt, EndAt: c.EndAt,
			AccountLinked: true, LinkURL: c.AccountLinkURL, Source: "catalog",
		}
		if len(c.Channels) > 0 {
			mc.AllowedChannels = []models.Channel{}
			for _, ch := range c.Channels {
				mc.AllowedChannels = append(mc.AllowedChannels, models.Channel{ID: ch.ID, Login: ch.Login, DisplayName: ch.DisplayName})
			}
		}
		for _, d := range c.Drops {
			if d.RequiredMinutes <= 0 {
				continue
			}
			mc.Drops = append(mc.Drops, models.Drop{
				ID: d.ID, Name: d.Name, Benefits: []string{d.Name}, ImageURL: d.ImageURL,
				RequiredMinutes: d.RequiredMinutes, StartAt: d.StartAt, EndAt: d.EndAt,
			})
		}
		if len(mc.Drops) > 0 {
			out = append(out, mc)
		}
	}
	return out
}
