package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/LmaoUpal/twitch_drops_rust/internal/catalog"
	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
	"github.com/LmaoUpal/twitch_drops_rust/internal/facepunch"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/auth"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/gql"
)

const campaignRefreshInterval = 15 * time.Minute

// GetCampaigns returns the last loaded campaign snapshot.
func (a *App) GetCampaigns() catalog.Snapshot {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.campaigns
}

// RefreshCampaigns asks the background loop to reload campaigns now.
func (a *App) RefreshCampaigns() {
	a.mu.Lock()
	ch := a.refreshNow
	a.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default: // a refresh is already queued
	}
}

// startCampaignLoop reloads campaigns now and every campaignRefreshInterval until logout.
func (a *App) startCampaignLoop(tok *auth.Token) {
	a.mu.Lock()
	if a.stopLoop != nil && a.loopForToken == tok.AccessToken {
		a.mu.Unlock()
		return
	}
	if a.stopLoop != nil {
		a.stopLoop()
	}
	ctx, cancel := context.WithCancel(a.ctx)
	refresh := make(chan struct{}, 1)
	a.stopLoop, a.refreshNow, a.loopForToken = cancel, refresh, tok.AccessToken
	a.mu.Unlock()

	client := a.gqlClient(tok)
	go func() {
		ticker := time.NewTicker(campaignRefreshInterval)
		defer ticker.Stop()
		for {
			a.loadCampaigns(ctx, client)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			case <-refresh:
			}
		}
	}()
}

func (a *App) stopCampaignLoop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopLoop != nil {
		a.stopLoop()
	}
	a.stopLoop, a.refreshNow, a.loopForToken = nil, nil, ""
	a.campaigns = catalog.Snapshot{}
}

func (a *App) loadCampaigns(ctx context.Context, client *gql.Client) {
	snap, err := catalog.Load(ctx, client, func(ctx context.Context) ([]facepunch.Campaign, error) {
		return facepunch.Fetch(ctx, nil)
	}, a.wantsRust())
	if errors.Is(err, context.Canceled) {
		return
	}
	if err != nil {
		slog.Warn("load campaigns", "err", err)
		snap.Warnings = append(snap.Warnings, err.Error())
		a.mu.Lock()
		snap.Campaigns = a.campaigns.Campaigns // keep showing the last good list
		a.mu.Unlock()
	}
	for _, w := range snap.Warnings {
		slog.Warn("campaign source", "warning", w)
	}
	a.mu.Lock()
	a.campaigns = snap
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, eventCampaigns, snap)
}

func (a *App) wantsRust() bool {
	if len(a.cfg.Games) == 0 {
		return true
	}
	for _, g := range a.cfg.Games {
		if strings.EqualFold(g, catalog.Rust.Name) {
			return true
		}
	}
	return false
}

func (a *App) gqlClient(tok *auth.Token) *gql.Client {
	return &gql.Client{
		ClientID:  a.cfg.Twitch.ClientID,
		UserAgent: a.cfg.Twitch.UserAgent,
		ClientURL: a.cfg.Twitch.ClientURL,
		DeviceID:  a.cfg.DeviceID,
		SessionID: a.sessionID,
		Token:     tok.AccessToken,
		Hashes:    a.cfg.Twitch.Hashes,
		Retries:   2,
	}
}

// ensureDeviceID gives this install a stable X-Device-Id: Twitch's unique_id cookie
// when reachable, a random one otherwise.
func (a *App) ensureDeviceID() {
	if a.cfg.DeviceID != "" {
		return
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	id, err := gql.FetchDeviceID(ctx, nil, a.cfg.Twitch.ClientURL, a.cfg.Twitch.UserAgent)
	if err != nil {
		slog.Warn("fetch device id, using random", "err", err)
		b := make([]byte, 16)
		rand.Read(b)
		id = hex.EncodeToString(b)
	}
	a.cfg.DeviceID = id
	if a.cfgPath == "" {
		return
	}
	if err := config.Save(a.cfgPath, a.cfg); err != nil {
		slog.Error("save config", "err", err)
	}
}
