package gql

import (
	"context"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

// Inventory returns campaigns the user already has progress in, with per-drop progress.
// It is the only campaign query Twitch still answers for app clients.
func (c *Client) Inventory(ctx context.Context) ([]models.Campaign, error) {
	var out struct {
		CurrentUser *struct {
			Inventory struct {
				DropCampaignsInProgress []inventoryCampaign `json:"dropCampaignsInProgress"`
			} `json:"inventory"`
		} `json:"currentUser"`
	}
	if err := c.Do(ctx, "Inventory", map[string]any{"fetchRewardCampaigns": false}, &out); err != nil {
		return nil, err
	}
	if out.CurrentUser == nil {
		return nil, ErrUnauthorized
	}
	campaigns := make([]models.Campaign, 0, len(out.CurrentUser.Inventory.DropCampaignsInProgress))
	for _, ic := range out.CurrentUser.Inventory.DropCampaignsInProgress {
		if ic.Game == nil {
			continue
		}
		campaigns = append(campaigns, ic.model())
	}
	return campaigns, nil
}

// StreamInfo returns a channel's identity, current category and live state.
// DropsEnabled is not part of this response and is left false.
func (c *Client) StreamInfo(ctx context.Context, login string) (models.Channel, error) {
	var out struct {
		User *struct {
			ID                string `json:"id"`
			Login             string `json:"login"`
			DisplayName       string `json:"displayName"`
			BroadcastSettings struct {
				Game *gqlGame `json:"game"`
			} `json:"broadcastSettings"`
			Stream *struct {
				ID           string `json:"id"`
				ViewersCount int    `json:"viewersCount"`
			} `json:"stream"`
		} `json:"user"`
	}
	if err := c.Do(ctx, "VideoPlayerStreamInfoOverlayChannel", map[string]any{"channel": login}, &out); err != nil {
		return models.Channel{}, err
	}
	if out.User == nil {
		return models.Channel{Login: login}, nil // no such channel
	}
	u := out.User
	ch := models.Channel{ID: u.ID, Login: u.Login, DisplayName: u.DisplayName}
	if g := u.BroadcastSettings.Game; g != nil {
		ch.GameID = g.ID
	}
	if u.Stream != nil {
		ch.Online = true
		ch.Viewers = u.Stream.ViewersCount
		ch.BroadcastID = u.Stream.ID
	}
	return ch, nil
}

// GameStreams lists live streams of a game (by directory slug) that have drops enabled.
func (c *Client) GameStreams(ctx context.Context, slug string, limit int) ([]models.Channel, error) {
	vars := map[string]any{
		"limit":              limit,
		"slug":               slug,
		"imageWidth":         50,
		"includeCostreaming": false,
		"sortTypeIsRecency":  false,
		"options": map[string]any{
			"broadcasterLanguages":   []string{},
			"freeformTags":           nil,
			"includeRestricted":      []string{"SUB_ONLY_LIVE"},
			"recommendationsContext": map[string]any{"platform": "web"},
			"sort":                   "RELEVANCE",
			"systemFilters":          []string{"DROPS_ENABLED"},
			"tags":                   []string{},
			"requestID":              "JIRA-VXP-2397",
		},
	}
	var out struct {
		Game *struct {
			ID      string `json:"id"`
			Streams struct {
				Edges []struct {
					Node struct {
						ID           string `json:"id"`
						ViewersCount int    `json:"viewersCount"`
						Broadcaster  *struct {
							ID          string `json:"id"`
							Login       string `json:"login"`
							DisplayName string `json:"displayName"`
						} `json:"broadcaster"`
					} `json:"node"`
				} `json:"edges"`
			} `json:"streams"`
		} `json:"game"`
	}
	if err := c.Do(ctx, "DirectoryPage_Game", vars, &out); err != nil {
		return nil, err
	}
	if out.Game == nil {
		return nil, nil
	}
	var channels []models.Channel
	for _, e := range out.Game.Streams.Edges {
		b := e.Node.Broadcaster
		if b == nil {
			continue
		}
		channels = append(channels, models.Channel{
			ID: b.ID, Login: b.Login, DisplayName: b.DisplayName,
			Online: true, GameID: out.Game.ID, DropsEnabled: true, Viewers: e.Node.ViewersCount, BroadcastID: e.Node.ID,
		})
	}
	return channels, nil
}

type gqlGame struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Slug        string `json:"slug"`
}

func (g gqlGame) model() models.Game {
	name := g.DisplayName
	if name == "" {
		name = g.Name
	}
	return models.Game{ID: g.ID, Name: name, Slug: g.Slug}
}

// inventoryCampaign mirrors Inventory.dropCampaignsInProgress[]. Field names follow
// TwitchDropsMiner; not yet seen live with real progress (account had none on 2026-10-21).
type inventoryCampaign struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	StartAt        time.Time `json:"startAt"`
	EndAt          time.Time `json:"endAt"`
	AccountLinkURL string    `json:"accountLinkURL"`
	Game           *gqlGame  `json:"game"`
	Self           *struct {
		IsAccountConnected bool `json:"isAccountConnected"`
	} `json:"self"`
	Allow *struct {
		IsEnabled bool `json:"isEnabled"`
		Channels  []struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			DisplayName string `json:"displayName"`
		} `json:"channels"`
	} `json:"allow"`
	TimeBasedDrops []struct {
		ID                     string    `json:"id"`
		Name                   string    `json:"name"`
		StartAt                time.Time `json:"startAt"`
		EndAt                  time.Time `json:"endAt"`
		RequiredMinutesWatched int       `json:"requiredMinutesWatched"`
		BenefitEdges           []struct {
			Benefit struct {
				Name          string `json:"name"`
				ImageAssetURL string `json:"imageAssetURL"`
			} `json:"benefit"`
		} `json:"benefitEdges"`
		Self *struct {
			CurrentMinutesWatched int    `json:"currentMinutesWatched"`
			IsClaimed             bool   `json:"isClaimed"`
			DropInstanceID        string `json:"dropInstanceID"`
		} `json:"self"`
	} `json:"timeBasedDrops"`
}

func (ic inventoryCampaign) model() models.Campaign {
	c := models.Campaign{
		ID: ic.ID, Name: ic.Name, Game: ic.Game.model(),
		StartAt: ic.StartAt, EndAt: ic.EndAt, LinkURL: ic.AccountLinkURL,
		AccountLinked: ic.Self == nil || ic.Self.IsAccountConnected,
		Source:        "twitch",
	}
	if ic.Allow != nil && ic.Allow.IsEnabled && len(ic.Allow.Channels) > 0 {
		c.AllowedChannels = []models.Channel{}
		for _, ch := range ic.Allow.Channels {
			c.AllowedChannels = append(c.AllowedChannels, models.Channel{ID: ch.ID, Login: ch.Name, DisplayName: ch.DisplayName})
		}
	}
	for _, d := range ic.TimeBasedDrops {
		drop := models.Drop{
			ID: d.ID, Name: d.Name, RequiredMinutes: d.RequiredMinutesWatched,
			StartAt: d.StartAt, EndAt: d.EndAt,
		}
		for _, b := range d.BenefitEdges {
			drop.Benefits = append(drop.Benefits, b.Benefit.Name)
			if drop.ImageURL == "" {
				drop.ImageURL = b.Benefit.ImageAssetURL
			}
		}
		if d.Self != nil {
			drop.CurrentMinutes = d.Self.CurrentMinutesWatched
			drop.Claimed = d.Self.IsClaimed
			drop.InstanceID = d.Self.DropInstanceID
		}
		c.Drops = append(c.Drops, drop)
	}
	return c
}

// DropSession is the drop currently progressing on a channel for this user.
type DropSession struct {
	DropID          string `json:"dropId"`
	ChannelID       string `json:"channelId"`
	CurrentMinutes  int    `json:"currentMinutes"`
	RequiredMinutes int    `json:"requiredMinutes"`
}

// CurrentDrop reports which drop watching channelID is progressing, if any.
func (c *Client) CurrentDrop(ctx context.Context, channelID string) (DropSession, error) {
	var out struct {
		CurrentUser *struct {
			DropCurrentSession *struct {
				Channel *struct {
					ID string `json:"id"`
				} `json:"channel"`
				CurrentMinutesWatched  int    `json:"currentMinutesWatched"`
				RequiredMinutesWatched int    `json:"requiredMinutesWatched"`
				DropID                 string `json:"dropID"`
			} `json:"dropCurrentSession"`
		} `json:"currentUser"`
	}
	if err := c.Do(ctx, "DropCurrentSessionContext", map[string]any{"channelID": channelID, "channelLogin": ""}, &out); err != nil {
		return DropSession{}, err
	}
	if out.CurrentUser == nil {
		return DropSession{}, ErrUnauthorized
	}
	s := out.CurrentUser.DropCurrentSession
	if s == nil {
		return DropSession{}, nil
	}
	ds := DropSession{DropID: s.DropID, CurrentMinutes: s.CurrentMinutesWatched, RequiredMinutes: s.RequiredMinutesWatched}
	if s.Channel != nil {
		ds.ChannelID = s.Channel.ID
	}
	return ds, nil
}
