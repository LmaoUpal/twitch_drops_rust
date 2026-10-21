package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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

// refreshInterval is how often campaigns are reloaded, from settings.
func (a *App) refreshInterval() time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	return time.Duration(max(a.cfg.RefreshSeconds, config.MinRefreshSeconds)) * time.Second
}

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

// startCampaignLoop loads the profile, then reloads campaigns now and every refreshInterval until logout.
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
	a.startAutoLoop(ctx, tok)
	go func() {
		a.loadProfile(ctx, client, tok.Login)
		for {
			a.loadCampaigns(ctx, client)
			select {
			case <-ctx.Done():
				return
			case <-time.After(a.refreshInterval()):
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
	a.stopLoop, a.refreshNow, a.loopForToken, a.autoKick = nil, nil, "", nil
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
		slog.Error("Couldn't load campaigns", "tag", "Twitch", "err", err)
		snap.Warnings = append(snap.Warnings, err.Error())
		a.mu.Lock()
		snap.Campaigns, snap.Claimed = a.campaigns.Campaigns, a.campaigns.Claimed // keep showing the last good list
		a.mu.Unlock()
	} else {
		slog.Info(fmt.Sprintf("Campaigns refreshed — %d campaigns", len(snap.Campaigns)), "tag", "Twitch")
	}
	for _, w := range snap.Warnings {
		slog.Warn(w, "tag", "Sources")
	}
	a.mu.Lock()
	a.campaigns = snap
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, eventCampaigns, snap)
	a.kickAuto()
}

// loadProfile fills in the signed-in user's avatar for the UI.
func (a *App) loadProfile(ctx context.Context, client *gql.Client, login string) {
	ch, err := client.StreamInfo(ctx, login)
	if err != nil {
		slog.Warn("Couldn't load profile", "tag", "Twitch", "err", err)
		return
	}
	a.mu.Lock()
	if a.status.State != authLoggedIn {
		a.mu.Unlock()
		return
	}
	a.status.AvatarURL, a.status.DisplayName = ch.AvatarURL, ch.DisplayName
	s := a.status
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, eventAuthStatus, s)
}

func (a *App) wantsRust() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
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
	a.mu.Lock()
	defer a.mu.Unlock()
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
	saveConfig(a.cfgPath, a.cfg)
}

func saveConfig(path string, cfg config.Config) {
	if err := config.Save(path, cfg); err != nil {
		slog.Error("Couldn't save settings", "tag", "Settings", "err", err)
	}
}
