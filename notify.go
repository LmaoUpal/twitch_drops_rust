package main

import (
	"log/slog"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/LmaoUpal/twitch_drops_rust/internal/catalog"
	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
)

// initNotifications prepares Windows toasts; failures only disable notifications.
func (a *App) initNotifications() {
	if err := runtime.InitializeNotifications(a.ctx); err != nil {
		slog.Warn("Desktop notifications unavailable", "tag", "App", "err", err)
		return
	}
	a.mu.Lock()
	a.notifyReady = true
	a.mu.Unlock()
}

// notify shows a desktop notification if notifications work on this system.
func (a *App) notify(title, body string) {
	a.mu.Lock()
	ready := a.notifyReady
	a.mu.Unlock()
	if !ready {
		return
	}
	err := runtime.SendNotification(a.ctx, runtime.NotificationOptions{
		ID:    title + time.Now().Format(time.RFC3339Nano),
		Title: title,
		Body:  body,
	})
	if err != nil {
		slog.Warn("Couldn't show notification", "tag", "App", "err", err)
	}
}

// notifyCampaignChanges announces campaigns that started and campaigns that need account
// linking. The first snapshot after login only records what is already known.
func (a *App) notifyCampaignChanges(snap catalog.Snapshot) {
	now := time.Now()
	a.mu.Lock()
	cfg := a.cfg
	first := a.seenActive == nil
	if first {
		a.seenActive, a.warnedUnlinked = map[string]bool{}, map[string]bool{}
	}
	var started, unlinked []models.Campaign
	for _, c := range snap.Campaigns {
		if !wantedGame(cfg.Games, c.Game.Name) || isStreamerChild(c) || now.Before(c.StartAt) || !now.Before(c.EndAt) {
			continue
		}
		if !a.seenActive[c.ID] {
			a.seenActive[c.ID] = true
			if !first {
				started = append(started, c)
			}
		}
		if !c.AccountLinked && !a.warnedUnlinked[c.ID] {
			a.warnedUnlinked[c.ID] = true
			unlinked = append(unlinked, c)
		}
	}
	a.mu.Unlock()

	if cfg.Notify.CampaignStarted {
		for _, c := range started {
			a.notify("Campaign started: "+c.Game.Name, c.Name)
		}
	}
	for _, c := range unlinked {
		slog.Warn(c.Game.Name+" account not linked — drops can't be claimed", "game", c.Game.Name)
		if cfg.Notify.AccountNotLinked {
			a.notify(c.Game.Name+" account not linked", "Link it to claim drops from "+c.Name+".")
		}
	}
}

func (a *App) notifyClaimed(drop, campaign string) {
	a.mu.Lock()
	on := a.cfg.Notify.DropClaimed
	a.mu.Unlock()
	if on {
		a.notify("Drop claimed: "+drop, campaign)
	}
}

func wantedGame(games []string, game string) bool {
	if len(games) == 0 {
		return true
	}
	for _, g := range games {
		if strings.EqualFold(g, game) {
			return true
		}
	}
	return false
}

// isStreamerChild reports a Facepunch streamer campaign, announced with its general one.
func isStreamerChild(c models.Campaign) bool {
	return c.Source == "facepunch" && !strings.HasSuffix(c.ID, "-general")
}
