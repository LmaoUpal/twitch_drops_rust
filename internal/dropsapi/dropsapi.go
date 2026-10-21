// Package dropsapi reads the community catalog of current Twitch drop campaigns from
// twitch-drops-api.sunkwi.com (github.com/SunkwiBOT/twitch-drops-api).
//
// Twitch no longer lists campaigns to app clients, and this app does not work around
// that gate itself. The catalog is a third-party service with no stated terms or
// collection method; it is used read-only for the "All drops" overview.
package dropsapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const URL = "https://twitch-drops-api.sunkwi.com/v2/drops"

type Game struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	BoxArtURL string `json:"boxArtUrl"`
}

type Channel struct {
	ID          string `json:"id"`
	Login       string `json:"login"`
	DisplayName string `json:"displayName"`
}

type Drop struct {
	ID              string    `json:"id"`
	Name            string    `json:"name"`
	ImageURL        string    `json:"imageUrl"`
	RequiredMinutes int       `json:"requiredMinutes"`
	RequiredSubs    int       `json:"requiredSubs"`
	StartAt         time.Time `json:"startAt"`
	EndAt           time.Time `json:"endAt"`
}

type Campaign struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Owner          string    `json:"owner"`
	Game           Game      `json:"game"`
	Status         string    `json:"status"`
	StartAt        time.Time `json:"startAt"`
	EndAt          time.Time `json:"endAt"`
	ImageURL       string    `json:"imageUrl"`
	DetailsURL     string    `json:"detailsUrl"`
	AccountLinkURL string    `json:"accountLinkUrl"`
	// Channels limits progress to these channels; empty means any channel of Game.
	Channels []Channel `json:"channels"`
	Drops    []Drop    `json:"drops"`
}

type Catalog struct {
	Campaigns []Campaign `json:"campaigns"`
	UpdatedAt time.Time  `json:"updatedAt"` // when the catalog last refreshed from Twitch
}

// Fetch downloads the catalog from url (URL in production).
func Fetch(ctx context.Context, httpClient *http.Client, url string) (Catalog, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return Catalog{}, err
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return Catalog{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Catalog{}, fmt.Errorf("dropsapi: http %d", resp.StatusCode)
	}
	return Parse(io.LimitReader(resp.Body, 32<<20))
}

// Parse decodes a /v2/drops response.
func Parse(r io.Reader) (Catalog, error) {
	var env struct {
		LastUpdatedAt time.Time  `json:"lastUpdatedAt"`
		Data          []gameJSON `json:"data"`
	}
	if err := json.NewDecoder(r).Decode(&env); err != nil {
		return Catalog{}, fmt.Errorf("dropsapi: decode: %w", err)
	}
	cat := Catalog{UpdatedAt: env.LastUpdatedAt, Campaigns: []Campaign{}}
	for _, g := range env.Data {
		for _, r := range g.Rewards {
			cat.Campaigns = append(cat.Campaigns, r.campaign(g))
		}
	}
	return cat, nil
}

type gameJSON struct {
	GameID          string       `json:"gameId"`
	GameDisplayName string       `json:"gameDisplayName"`
	GameBoxArtURL   string       `json:"gameBoxArtURL"`
	Rewards         []rewardJSON `json:"rewards"`
}

type rewardJSON struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Status         string    `json:"status"`
	StartAt        time.Time `json:"startAt"`
	EndAt          time.Time `json:"endAt"`
	ImageURL       string    `json:"imageURL"`
	DetailsURL     string    `json:"detailsURL"`
	AccountLinkURL string    `json:"accountLinkURL"`
	Owner          *struct {
		Name string `json:"name"`
	} `json:"owner"`
	Game *struct {
		Slug string `json:"slug"`
	} `json:"game"`
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
		RequiredMinutesWatched int       `json:"requiredMinutesWatched"`
		RequiredSubs           int       `json:"requiredSubs"`
		StartAt                time.Time `json:"startAt"`
		EndAt                  time.Time `json:"endAt"`
		BenefitEdges           []struct {
			Benefit struct {
				Name          string `json:"name"`
				ImageAssetURL string `json:"imageAssetURL"`
			} `json:"benefit"`
		} `json:"benefitEdges"`
	} `json:"timeBasedDrops"`
}

func (r rewardJSON) campaign(g gameJSON) Campaign {
	c := Campaign{
		ID: r.ID, Name: r.Name, Status: r.Status, StartAt: r.StartAt, EndAt: r.EndAt,
		ImageURL: r.ImageURL, DetailsURL: r.DetailsURL, AccountLinkURL: r.AccountLinkURL,
		Game:     Game{ID: g.GameID, Name: g.GameDisplayName, BoxArtURL: g.GameBoxArtURL},
		Channels: []Channel{},
		Drops:    []Drop{},
	}
	if r.Owner != nil {
		c.Owner = r.Owner.Name
	}
	if r.Game != nil {
		c.Game.Slug = r.Game.Slug
	}
	if r.Allow != nil && r.Allow.IsEnabled {
		for _, ch := range r.Allow.Channels {
			c.Channels = append(c.Channels, Channel{ID: ch.ID, Login: ch.Name, DisplayName: ch.DisplayName})
		}
	}
	for _, d := range r.TimeBasedDrops {
		drop := Drop{ID: d.ID, Name: d.Name, RequiredMinutes: d.RequiredMinutesWatched, RequiredSubs: d.RequiredSubs, StartAt: d.StartAt, EndAt: d.EndAt}
		if len(d.BenefitEdges) > 0 {
			drop.ImageURL = d.BenefitEdges[0].Benefit.ImageAssetURL
			if n := d.BenefitEdges[0].Benefit.Name; n != "" {
				drop.Name = n
			}
		}
		c.Drops = append(c.Drops, drop)
	}
	return c
}
