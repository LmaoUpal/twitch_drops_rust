package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/LmaoUpal/twitch_drops_rust/internal/miner"
	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
	"github.com/LmaoUpal/twitch_drops_rust/internal/scheduler"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/auth"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/gql"
)

const (
	eventAuto = "auto:status"

	modeAuto   = "auto"
	modeManual = "manual"

	autoInterval    = time.Minute      // how often auto mode re-decides without other triggers
	liveCacheTTL    = 5 * time.Minute  // channel and directory lookups are reused this long
	exploreDuration = 10 * time.Minute // how long to watch a game with no known campaign
	exploreCooldown = 6 * time.Hour    // before exploring the same game again
	streamsPerGame  = 30
	verifyAfter     = 4 * time.Minute  // a queued channel must show drop progress by then
	noDropsTTL      = 30 * time.Minute // a queued channel without drop progress is skipped this long
)

type QueueItem struct {
	Channel     models.Channel `json:"channel"`
	GameName    string         `json:"gameName"`
	Campaign    string         `json:"campaign"`
	Drop        string         `json:"drop"`
	DropCurrent int            `json:"dropCurrent"`
	DropNeeded  int            `json:"dropNeeded"`
	MinutesLeft int            `json:"minutesLeft"`
	Reason      string         `json:"reason"`
}

type AutoState struct {
	Mode      string      `json:"mode"` // "auto" or "manual"
	Paused    bool        `json:"paused"`
	State     string      `json:"state"` // idle | watching | exploring | paused | manual
	Exploring string      `json:"exploring"`
	Queue     []QueueItem `json:"queue"`
	NextCheck time.Time   `json:"nextCheck"`
	LastError string      `json:"lastError"`
}

type cached[T any] struct {
	value T
	at    time.Time
}

// autoRun is the state of one auto-mode loop, owned by its goroutine.
type autoRun struct {
	client       *gql.Client
	channels     map[string]cached[models.Channel]
	streams      map[string]cached[[]models.Channel]
	explored     map[string]time.Time // game name -> when exploring it ended
	exploreGame  string
	exploreUntil time.Time
	claimed      map[string]bool      // claim instance ids already attempted
	skip         map[string]time.Time // channels that turned out offline, ignored for liveCacheTTL
	// unverified holds queued channels found only by login: Twitch doesn't say whether their
	// stream has drops enabled, so the watch is checked for drop progress after verifyAfter.
	unverified map[string]bool
	noDrops    map[string]time.Time // queued channels whose watch showed no drop progress
}

// GetAutoState returns the auto-mode status for the UI.
func (a *App) GetAutoState() AutoState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.auto
}

// SetAutoMode switches between auto and manual mode and persists the choice.
func (a *App) SetAutoMode(auto bool) {
	a.setMode(auto)
	if auto {
		a.SetPaused(false)
	}
	a.kickAuto()
}

// SetPaused pauses or resumes auto mode. Pausing stops the current watch.
func (a *App) SetPaused(paused bool) {
	a.setAuto(func(s *AutoState) { s.Paused = paused })
	if paused {
		slog.Info("Mining paused", "tag", "App")
		a.stopWatching()
	} else {
		slog.Info("Mining resumed", "tag", "App")
	}
	a.kickAuto()
}

// ClaimDrop claims one completed drop now (the manual Claim button).
func (a *App) ClaimDrop(instanceID string) error {
	a.mu.Lock()
	tok := a.token
	a.mu.Unlock()
	if tok == nil {
		return fmt.Errorf("not logged in")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	for _, it := range miner.Claimable(a.GetCampaigns().Campaigns) {
		if it.Drop.InstanceID == instanceID {
			err := a.claim(ctx, a.gqlClient(tok), it)
			a.RefreshCampaigns()
			return err
		}
	}
	return fmt.Errorf("drop is not ready to claim")
}

func (a *App) setMode(auto bool) {
	mode := modeManual
	if auto {
		mode = modeAuto
	}
	a.mu.Lock()
	changed := a.cfg.AutoMode != auto
	a.cfg.AutoMode = auto
	cfg, path := a.cfg, a.cfgPath
	a.mu.Unlock()
	a.setAuto(func(s *AutoState) {
		s.Mode = mode
		if !auto {
			s.State, s.Queue, s.Exploring = modeManual, nil, ""
		}
	})
	if changed {
		slog.Info("Mode: "+mode, "tag", "App")
		if path != "" {
			saveConfig(path, cfg)
		}
	}
}

// kickAuto makes the auto loop re-decide now instead of waiting for its timer.
func (a *App) kickAuto() {
	a.mu.Lock()
	ch := a.autoKick
	a.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (a *App) setAuto(update func(*AutoState)) {
	a.mu.Lock()
	update(&a.auto)
	s := a.auto
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, eventAuto, s)
	a.updateTray()
}

// startAutoLoop runs auto mode (and auto-claim, which works in both modes) until ctx ends.
func (a *App) startAutoLoop(ctx context.Context, tok *auth.Token) {
	kick := make(chan struct{}, 1)
	a.mu.Lock()
	a.autoKick = kick
	auto := a.cfg.AutoMode
	a.mu.Unlock()
	a.setMode(auto)

	run := &autoRun{
		client:     a.gqlClient(tok),
		channels:   map[string]cached[models.Channel]{},
		streams:    map[string]cached[[]models.Channel]{},
		explored:   map[string]time.Time{},
		claimed:    map[string]bool{},
		skip:       map[string]time.Time{},
		unverified: map[string]bool{},
		noDrops:    map[string]time.Time{},
	}
	go func() {
		for {
			a.autoStep(ctx, run)
			a.setAuto(func(s *AutoState) { s.NextCheck = time.Now().Add(autoInterval) })
			select {
			case <-ctx.Done():
				return
			case <-kick:
			case <-time.After(autoInterval):
			}
		}
	}()
}

func (a *App) autoStep(ctx context.Context, run *autoRun) {
	snap := a.GetCampaigns()
	if snap.UpdatedAt.IsZero() {
		return // campaigns not loaded yet; deciding now would explore games that do have campaigns
	}
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock()

	if cfg.AutoClaim {
		claimedAny := false
		for _, it := range miner.Claimable(snap.Campaigns) {
			if run.claimed[it.Drop.InstanceID] {
				continue
			}
			run.claimed[it.Drop.InstanceID] = true
			if a.claim(ctx, run.client, it) == nil {
				claimedAny = true
			}
		}
		if claimedAny {
			a.RefreshCampaigns()
		}
	}

	st := a.GetAutoState()
	if st.Mode != modeAuto {
		return
	}
	if st.Paused {
		a.setAuto(func(s *AutoState) { s.State, s.Queue, s.Exploring = "paused", nil, "" })
		return
	}

	now := time.Now()
	prefs := scheduler.Prefs{Games: cfg.Games, ExcludedChannels: cfg.ExcludedChannels, Preferred: cfg.PreferredChannels}
	live, lastErr := a.liveChannels(ctx, run, snap.Campaigns, prefs, now)

	watch := a.GetWatchState()
	run.verifyDrops(watch, now)
	live = run.withoutSkipped(live, now)
	var current *models.Channel
	if (watch.State == watchActive || watch.State == watchStarting) && !run.hasNoDrops(watch.Channel.Login, now) {
		ch := watch.Channel
		if fresh, err := run.channel(ctx, ch.Login, now); err == nil {
			ch.GameID, ch.Online = fresh.GameID, fresh.Online
		}
		ch.DropsEnabled = true // it was picked from a drops-enabled listing
		current = &ch
		live = append(live, ch)
	}

	d := miner.Decide(now, snap.Campaigns, live, prefs, current)
	queue := queueItems(d.Queue)

	switch {
	case d.Action == miner.Keep:
		run.exploreGame = "" // exploring found a campaign on this channel
	case d.Action == miner.Switch:
		run.exploreGame = ""
		a.autoWatch(run, d.Target.Channel.Login, d.Target.Reason, &lastErr)
	case d.Action == miner.Idle && run.exploreGame != "" && now.Before(run.exploreUntil) && current != nil && wanted(prefs, run.exploreGame):
		// keep exploring
	case d.Action == miner.Idle:
		if run.exploreGame != "" {
			slog.Info("Explored "+run.exploreGame+": no campaign showed up", "game", run.exploreGame)
			run.explored[strings.ToLower(run.exploreGame)] = now
			run.exploreGame = ""
		}
		if game, ch := a.pickExplore(ctx, run, snap.Campaigns, prefs, now); ch != nil {
			run.exploreGame, run.exploreUntil = game, now.Add(exploreDuration)
			a.autoWatch(run, ch.Login, "exploring "+game, &lastErr)
		} else if watch.State == watchActive || watch.State == watchStarting {
			slog.Info("Nothing left to watch", "tag", "App")
			a.stopWatching()
		}
	}

	state := "idle"
	if w := a.GetWatchState(); w.State == watchActive || w.State == watchStarting {
		state = "watching"
		if run.exploreGame != "" {
			state = "exploring"
		}
	}
	a.setAuto(func(s *AutoState) {
		s.State, s.Queue, s.Exploring, s.LastError = state, queue, run.exploreGame, lastErr
	})
}

// liveChannels lists candidate channels: everyone on a short campaign allow list plus live
// drops-enabled streams of games with open campaigns or long allow lists (scheduler then
// keeps only allowed channels).
func (a *App) liveChannels(ctx context.Context, run *autoRun, campaigns []models.Campaign, prefs scheduler.Prefs, now time.Time) ([]models.Channel, string) {
	var live []models.Channel
	var lastErr string
	for _, c := range scheduler.Eligible(now, campaigns, prefs) {
		if !c.LookupChannels() {
			continue
		}
		for _, ch := range c.AllowedChannels {
			fresh, err := run.channel(ctx, ch.Login, now)
			if err != nil {
				lastErr = err.Error()
				continue
			}
			fresh.DropsEnabled = true
			live = append(live, fresh)
		}
	}
	for _, g := range miner.GamesToScan(now, campaigns, prefs) {
		streams, err := run.gameStreams(ctx, miner.Slug(g), now)
		if err != nil {
			lastErr = err.Error()
			continue
		}
		live = append(live, streams...)
	}
	// Queued channels go last: one already listed above keeps its drops-enabled entry
	// (scheduler takes the first occurrence of a login).
	listed := map[string]bool{}
	for _, ch := range live {
		listed[strings.ToLower(ch.Login)] = true
	}
	seenGame := map[string]bool{}
	for _, c := range scheduler.Eligible(now, campaigns, prefs) {
		if seenGame[c.Game.ID] {
			continue
		}
		seenGame[c.Game.ID] = true
		for _, login := range scheduler.Queue(prefs, c.Game.Name) {
			key := strings.ToLower(login)
			if listed[key] {
				continue
			}
			fresh, err := run.channel(ctx, login, now)
			if err != nil {
				lastErr = err.Error()
				continue
			}
			listed[key] = true
			fresh.DropsEnabled = true
			run.unverified[key] = true
			live = append(live, fresh)
		}
	}
	return live, lastErr
}

// verifyDrops checks the watch of a queued channel that wasn't listed as drops-enabled:
// without drop progress after verifyAfter its stream has no drops, so it is skipped for a while.
func (r *autoRun) verifyDrops(w WatchState, now time.Time) {
	key := strings.ToLower(w.Channel.Login)
	if w.State != watchActive || !r.unverified[key] || now.Sub(w.Started) < verifyAfter {
		return
	}
	delete(r.unverified, key)
	if w.Session.DropID == "" {
		slog.Warn("No drop progress on "+w.Channel.DisplayName+": drops seem disabled on this stream, skipping it for now", "channel", w.Channel.DisplayName)
		r.noDrops[key] = now
	}
}

func (r *autoRun) hasNoDrops(login string, now time.Time) bool {
	t, ok := r.noDrops[strings.ToLower(login)]
	return ok && now.Sub(t) < noDropsTTL
}

// pickExplore returns a wanted game with no known campaign and a live drops stream of it.
func (a *App) pickExplore(ctx context.Context, run *autoRun, campaigns []models.Campaign, prefs scheduler.Prefs, now time.Time) (string, *models.Channel) {
	for _, game := range miner.Unexplored(now, campaigns, prefs.Games) {
		if t, ok := run.explored[strings.ToLower(game)]; ok && now.Sub(t) < exploreCooldown {
			continue
		}
		streams, err := run.gameStreams(ctx, miner.Slug(models.Game{Name: game}), now)
		if err != nil {
			continue
		}
		for _, ch := range streams {
			if !scheduler.Excluded(prefs, ch.Login) {
				slog.Info("Exploring "+game+": watching a drops stream to discover campaigns", "game", game)
				return game, &ch
			}
		}
		run.explored[strings.ToLower(game)] = now // no live drops streams right now
	}
	return "", nil
}

// autoWatch starts watching login for auto mode; a channel that turns out offline is skipped
// for a while so stale listings don't make the loop retry it every step.
func (a *App) autoWatch(run *autoRun, login, reason string, lastErr *string) {
	if err := a.watchLogin(login, reason); err != nil {
		*lastErr = err.Error()
	}
	if w := a.GetWatchState(); w.State != watchActive && w.State != watchStarting {
		run.skip[strings.ToLower(login)] = time.Now()
		delete(run.channels, strings.ToLower(login))
	}
}

func (a *App) claim(ctx context.Context, client *gql.Client, it miner.ClaimItem) error {
	status, err := client.ClaimDrop(ctx, it.Drop.InstanceID)
	if err != nil {
		slog.Error("Claim failed: "+it.Drop.Name, "game", it.Campaign.Game.Name, "err", err)
		return err
	}
	slog.Info(fmt.Sprintf("Claimed: %s — %s", it.Drop.Name, it.Campaign.Name), "game", it.Campaign.Game.Name, "ok", true)
	a.notifyClaimed(it.Drop.Name, it.Campaign.Name)
	if status != "" && status != "ELIGIBLE_FOR_ALL" {
		slog.Info("Claim status: "+status, "game", it.Campaign.Game.Name)
	}
	return nil
}

func (r *autoRun) channel(ctx context.Context, login string, now time.Time) (models.Channel, error) {
	key := strings.ToLower(login)
	if c, ok := r.channels[key]; ok && now.Sub(c.at) < liveCacheTTL {
		return c.value, nil
	}
	ch, err := r.client.StreamInfo(ctx, login)
	if err != nil {
		return ch, err
	}
	r.channels[key] = cached[models.Channel]{ch, now}
	return ch, nil
}

func (r *autoRun) gameStreams(ctx context.Context, slug string, now time.Time) ([]models.Channel, error) {
	if c, ok := r.streams[slug]; ok && now.Sub(c.at) < liveCacheTTL {
		return c.value, nil
	}
	streams, err := r.client.GameStreams(ctx, slug, streamsPerGame)
	if err != nil {
		return nil, err
	}
	r.streams[slug] = cached[[]models.Channel]{streams, now}
	return streams, nil
}

func wanted(prefs scheduler.Prefs, game string) bool {
	for _, g := range prefs.Games {
		if strings.EqualFold(g, game) {
			return true
		}
	}
	return false
}

func (r *autoRun) withoutSkipped(live []models.Channel, now time.Time) []models.Channel {
	out := live[:0:0]
	for _, ch := range live {
		if t, ok := r.skip[strings.ToLower(ch.Login)]; ok && now.Sub(t) < liveCacheTTL {
			continue
		}
		if r.hasNoDrops(ch.Login, now) {
			continue
		}
		out = append(out, ch)
	}
	return out
}

func queueItems(choices []scheduler.Choice) []QueueItem {
	out := []QueueItem{}
	for _, c := range choices {
		it := QueueItem{Channel: c.Channel, MinutesLeft: c.MinutesLeft, Reason: c.Reason}
		if len(c.Campaigns) > 0 {
			top := c.Campaigns[0]
			it.GameName, it.Campaign = top.Game.Name, top.Name
			for _, d := range top.Drops {
				if d.RequiredMinutes > d.CurrentMinutes && !d.Claimed {
					it.Drop, it.DropCurrent, it.DropNeeded = d.Name, d.CurrentMinutes, d.RequiredMinutes
					break
				}
			}
		}
		out = append(out, it)
	}
	return out
}
