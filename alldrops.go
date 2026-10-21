package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/dropsapi"
)

const allDropsTTL = 10 * time.Minute

type AllDrops struct {
	Campaigns []dropsapi.Campaign `json:"campaigns"`
	UpdatedAt time.Time           `json:"updatedAt"` // catalog's own refresh time
	FetchedAt time.Time           `json:"fetchedAt"`
	Source    string              `json:"source"`
	Error     string              `json:"error"`
}

// GetAllDrops returns every current Twitch drop campaign from the community catalog,
// cached for allDropsTTL unless refresh is set. On failure the last good list is kept.
func (a *App) GetAllDrops(refresh bool) AllDrops {
	a.mu.Lock()
	cached := a.allDrops
	a.mu.Unlock()
	if !refresh && !cached.FetchedAt.IsZero() && time.Since(cached.FetchedAt) < allDropsTTL {
		return cached
	}

	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	cat, err := dropsapi.Fetch(ctx, nil, dropsapi.URL)
	res := AllDrops{Source: dropsapi.URL, FetchedAt: time.Now()}
	if err != nil {
		slog.Warn("Couldn't load the all-drops catalog", "tag", "Catalog", "err", err)
		res.Campaigns, res.UpdatedAt, res.Error = cached.Campaigns, cached.UpdatedAt, err.Error()
		if res.Campaigns == nil {
			res.Campaigns = []dropsapi.Campaign{}
		}
	} else {
		slog.Info(fmt.Sprintf("All-drops catalog: %d campaigns", len(cat.Campaigns)), "tag", "Catalog")
		res.Campaigns, res.UpdatedAt = cat.Campaigns, cat.UpdatedAt
	}
	a.mu.Lock()
	a.allDrops = res
	a.mu.Unlock()
	return res
}
