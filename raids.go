package main

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/realtime"
)

// raidWatchFor is how long the raided channel is watched after a listed channel raids it.
const raidWatchFor = 10 * time.Minute

// raidWatch is the channel a listed channel raided, watched in a free points slot until until.
type raidWatch struct {
	login string
	until time.Time
}

// onRaid joins raids started by listed channels and remembers the raided channel.
func (a *App) onRaid(ctx context.Context, run *pointsRun, kind string, r realtime.Raid) {
	a.mu.Lock()
	join := a.cfg.Points.Enabled && a.cfg.Points.JoinRaids
	a.mu.Unlock()
	src, ok := a.pointsChannelByID(r.SourceID)
	if !join || !ok {
		return
	}
	from := src.DisplayName
	switch kind {
	case "raid_update_v2", "raid_go_v2":
		run.mu.Lock()
		first := !run.raids[r.ID]
		run.raids[r.ID] = true
		if first {
			// Raids last ~90 s from announcement; the watch covers the move and a while after.
			run.raid = raidWatch{login: strings.ToLower(r.TargetLogin), until: time.Now().Add(raidWatchFor + 2*time.Minute)}
		}
		run.mu.Unlock()
		if first {
			go func() {
				if err := run.client.JoinRaid(ctx, r.ID); err != nil {
					slog.Warn(fmt.Sprintf("Couldn't join the raid from %s to %s", from, r.TargetName), "channel", from, "err", err)
					return
				}
				slog.Info(fmt.Sprintf("Joined the raid from %s to %s", from, r.TargetName), "channel", from, "ok", true)
			}()
		}
		if kind == "raid_go_v2" {
			a.kickPoints()
		}
	case "raid_cancel_v2":
		run.mu.Lock()
		if run.raid.login == strings.ToLower(r.TargetLogin) {
			run.raid = raidWatch{}
		}
		run.mu.Unlock()
		slog.Info(fmt.Sprintf("%s cancelled the raid to %s", from, r.TargetName), "channel", from)
	}
}

// raidTarget returns the raided channel to watch now, if one is due and live and not on the
// list already.
func (a *App) raidTarget(ctx context.Context, run *pointsRun, listed []string, now time.Time) (models.Channel, bool) {
	run.mu.Lock()
	rw := run.raid
	run.mu.Unlock()
	if rw.login == "" || !now.Before(rw.until) || slices.ContainsFunc(listed, func(l string) bool { return strings.EqualFold(l, rw.login) }) {
		return models.Channel{}, false
	}
	ch, err := run.client.StreamInfo(ctx, rw.login)
	if err != nil || !ch.Online {
		return models.Channel{}, false
	}
	return ch, true
}
