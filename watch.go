package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/gql"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/watcher"
)

const (
	eventWatch = "watch:status"

	watchIdle     = "idle"
	watchStarting = "starting"
	watchActive   = "watching"
	watchOffline  = "offline"
	watchError    = "error"

	dropSessionInterval = time.Minute
)

type WatchState struct {
	State   string          `json:"state"`
	Channel models.Channel  `json:"channel"`
	Status  watcher.Status  `json:"status"`
	Session gql.DropSession `json:"session"` // what Twitch says this watch is progressing
	Reason  string          `json:"reason"`  // why auto mode picked this channel; empty for manual
	Started time.Time       `json:"started"`
	Error   string          `json:"error"`
}

// GetWatchState returns what is being watched right now.
func (a *App) GetWatchState() WatchState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.watch
}

// WatchChannel is the user picking a channel: it switches to manual mode and watches login.
func (a *App) WatchChannel(login string) error {
	a.setMode(false)
	return a.watchLogin(login, "")
}

// watchLogin stops any current watch and starts watching login. reason is shown in the UI.
func (a *App) watchLogin(login, reason string) error {
	login = strings.ToLower(strings.TrimSpace(login))
	a.stopWatching()
	a.mu.Lock()
	tok := a.token
	a.mu.Unlock()
	if tok == nil {
		return errors.New("not logged in")
	}
	client := a.gqlClient(tok)

	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.stopWatch = cancel
	a.mu.Unlock()
	a.setWatch(func(s *WatchState) {
		*s = WatchState{State: watchStarting, Channel: models.Channel{Login: login}, Reason: reason, Started: time.Now()}
	})

	ch, err := client.StreamInfo(ctx, login)
	if err != nil {
		cancel()
		a.setWatch(func(s *WatchState) { s.State, s.Error = watchError, err.Error() })
		return err
	}
	if !ch.Online {
		cancel()
		a.setWatch(func(s *WatchState) { s.State, s.Channel = watchOffline, ch })
		a.kickAuto()
		return nil
	}
	msg := "Watching " + ch.DisplayName
	if reason != "" {
		msg = fmt.Sprintf("Switched to %s (%s)", ch.DisplayName, reason)
	}
	slog.Info(msg, "channel", ch.DisplayName, "ok", true)
	a.setWatch(func(s *WatchState) { s.State, s.Channel = watchActive, ch })

	w := &watcher.Watcher{
		GQL: client, UserID: tok.UserID, UserAgent: client.UserAgent,
		Report: func(st watcher.Status) {
			if ctx.Err() == nil { // a stopped watch must not overwrite the next state
				a.setWatch(func(s *WatchState) { s.Status = st })
			}
		},
	}
	go a.trackDropSession(ctx, client, ch.ID)
	go func() {
		err := w.Watch(ctx, watcher.Stream{Channel: ch})
		switch {
		case errors.Is(err, context.Canceled):
		case errors.Is(err, watcher.ErrOffline):
			slog.Warn("Channel offline: "+ch.DisplayName, "channel", ch.DisplayName)
			a.setWatch(func(s *WatchState) { s.State = watchOffline })
		default:
			slog.Error("Watching stopped", "channel", ch.DisplayName, "err", err)
			a.setWatch(func(s *WatchState) { s.State, s.Error = watchError, err.Error() })
		}
		cancel()
		a.kickAuto()
	}()
	return nil
}

// StopWatching is the user pressing Stop. In auto mode it pauses mining, otherwise the
// miner would pick a channel again right away.
func (a *App) StopWatching() {
	if a.GetAutoState().Mode == modeAuto {
		a.SetPaused(true)
	}
	a.stopWatching()
}

// stopWatching stops the current watch, if any.
func (a *App) stopWatching() {
	a.mu.Lock()
	stop := a.stopWatch
	a.stopWatch = nil
	a.mu.Unlock()
	if stop != nil {
		stop()
		a.setWatch(func(s *WatchState) { *s = WatchState{State: watchIdle} })
	}
}

// trackDropSession polls which drop the watch progresses until ctx ends.
func (a *App) trackDropSession(ctx context.Context, client *gql.Client, channelID string) {
	t := time.NewTicker(dropSessionInterval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
		s, err := client.CurrentDrop(ctx, channelID)
		if err != nil {
			if ctx.Err() == nil {
				slog.Warn("Couldn't check drop progress", "tag", "Twitch", "err", err)
			}
			continue
		}
		if ctx.Err() != nil {
			return
		}
		prev := a.GetWatchState().Session
		if s.DropID != "" && (s.DropID != prev.DropID || s.CurrentMinutes != prev.CurrentMinutes) {
			slog.Info(fmt.Sprintf("Drop progress %d / %d min", s.CurrentMinutes, s.RequiredMinutes), "tag", "Twitch")
		}
		// A finished drop gets its claim instance in the inventory; reload so it can be claimed
		// and the miner moves on.
		if s.RequiredMinutes > 0 && s.CurrentMinutes >= s.RequiredMinutes && (prev.DropID != s.DropID || prev.CurrentMinutes < prev.RequiredMinutes) {
			slog.Info("Drop complete — refreshing inventory to claim it", "tag", "Twitch", "ok", true)
			a.RefreshCampaigns()
		}
		a.setWatch(func(st *WatchState) { st.Session = s })
	}
}

func (a *App) setWatch(update func(*WatchState)) {
	a.mu.Lock()
	update(&a.watch)
	s := a.watch
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, eventWatch, s)
}
