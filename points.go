package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
	"github.com/LmaoUpal/twitch_drops_rust/internal/points"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/auth"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/chat"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/gql"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/realtime"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/watcher"
)

const (
	eventPoints = "points:status"

	pointsInterval   = 2 * time.Minute  // how often live state is re-checked without PubSub events
	balanceInterval  = 15 * time.Minute // balance and pending bonus are re-read this often
	streamUpDelay    = 45 * time.Second // Twitch's API lags behind the stream-up event
	watchCooldown    = 5 * time.Minute  // a channel whose points watch ended on its own waits this long
	pointsWatching   = "watching"       // watched for points
	pointsDrops      = "drops"          // the drops watch is on this channel
	pointsLive       = "live"           // live, but no free slot
	pointsOffline    = "offline"
	pointsNotChecked = "unknown"
	pointsDisabled   = "disabled" // the streamer turned channel points off
)

type PointsChannelState struct {
	Channel   models.Channel `json:"channel"`
	State     string         `json:"state"` // watching | drops | live | offline | unknown
	Balance   int            `json:"balance"`
	Earned    int            `json:"earned"`    // points gained since the app started
	Bonuses   int            `json:"bonuses"`   // bonus chests claimed since the app started
	Bets      int            `json:"bets"`      // predictions bet on since the app started
	BetProfit int            `json:"betProfit"` // points won minus points bet, over resolved bets
	Error     string         `json:"error"`
}

type PointsState struct {
	Enabled  bool                 `json:"enabled"`
	Slots    int                  `json:"slots"` // channels watched for points right now at most
	Channels []PointsChannelState `json:"channels"`
	// BetsNeedTOS is set when Twitch refused a bet until the predictions terms are accepted.
	BetsNeedTOS bool `json:"betsNeedTos"`
}

// pointsRun is the state of one points loop. watchers is shared with watch goroutines.
type pointsRun struct {
	client   *gql.Client
	userID   string
	pubsub   *realtime.Client
	chat     *chat.Client
	checked  map[string]time.Time // login -> last balance read
	noPoints map[string]bool      // logins whose channel points are turned off

	mu       sync.Mutex
	watchers map[string]context.CancelFunc // login -> stop
	cooldown map[string]time.Time          // login -> when its watch ended on its own
	bets     map[string]*pendingBet        // prediction id -> bet scheduled or placed
}

// GetPointsState returns the channel points status for the UI.
func (a *App) GetPointsState() PointsState {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.points
}

// AddPointsChannel adds a channel to the end of the points list and follows it on Twitch
// when the follow option is on.
func (a *App) AddPointsChannel(login string) error {
	login = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(login), "@"))
	if login == "" {
		return errors.New("enter a channel name")
	}
	client, err := a.loggedInClient()
	if err != nil {
		return err
	}
	if slices.ContainsFunc(a.pointsLogins(), func(l string) bool { return l == login }) {
		return nil
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	ch, err := client.StreamInfo(ctx, login)
	if err != nil {
		return err
	}
	if ch.ID == "" {
		return fmt.Errorf("channel %s not found", login)
	}
	a.editPointsChannels(func(list []config.PointsChannel) []config.PointsChannel {
		return append(list, config.PointsChannel{Login: ch.Login})
	})
	slog.Info("Points: added "+ch.DisplayName, "channel", ch.DisplayName)
	a.mu.Lock()
	follow := a.cfg.Points.FollowOnAdd
	a.mu.Unlock()
	if follow {
		a.follow(ctx, client, ch)
	}
	a.kickPoints()
	return nil
}

// RemovePointsChannel stops farming points on a channel. It stays followed on Twitch.
func (a *App) RemovePointsChannel(login string) {
	a.editPointsChannels(func(list []config.PointsChannel) []config.PointsChannel {
		return slices.DeleteFunc(list, func(c config.PointsChannel) bool { return strings.EqualFold(c.Login, login) })
	})
	a.kickPoints()
}

// MovePointsChannel moves a channel to position to (0 is the highest priority).
func (a *App) MovePointsChannel(login string, to int) {
	a.editPointsChannels(func(list []config.PointsChannel) []config.PointsChannel {
		i := slices.IndexFunc(list, func(c config.PointsChannel) bool { return strings.EqualFold(c.Login, login) })
		if i < 0 {
			return list
		}
		c := list[i]
		list = slices.Delete(list, i, i+1)
		return slices.Insert(list, max(0, min(to, len(list))), c)
	})
	a.kickPoints()
}

// ImportFollows adds every followed channel not yet on the points list and returns how many
// were added.
func (a *App) ImportFollows() (int, error) {
	client, err := a.loggedInClient()
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, time.Minute)
	defer cancel()
	follows, err := client.Follows(ctx)
	if err != nil {
		return 0, err
	}
	added := 0
	a.editPointsChannels(func(list []config.PointsChannel) []config.PointsChannel {
		for _, f := range follows {
			if !slices.ContainsFunc(list, func(c config.PointsChannel) bool { return strings.EqualFold(c.Login, f.Login) }) {
				list = append(list, config.PointsChannel{Login: strings.ToLower(f.Login)})
				added++
			}
		}
		return list
	})
	slog.Info(fmt.Sprintf("Points: imported %d followed channels", added), "tag", "Points")
	a.kickPoints()
	return added, nil
}

// follow follows ch unless it is already followed: following again resets the follow date.
func (a *App) follow(ctx context.Context, client *gql.Client, ch models.Channel) {
	following, err := client.IsFollowing(ctx, ch.Login)
	if err == nil && !following {
		err = client.Follow(ctx, ch.ID)
		if err == nil {
			slog.Info("Followed "+ch.DisplayName, "channel", ch.DisplayName, "ok", true)
		}
	}
	if err != nil {
		slog.Warn("Couldn't follow "+ch.DisplayName, "channel", ch.DisplayName, "err", err)
	}
}

// editPointsChannels changes the channel list in the config, saves it and updates the UI list.
func (a *App) editPointsChannels(edit func([]config.PointsChannel) []config.PointsChannel) {
	a.mu.Lock()
	a.cfg.Points.Channels = edit(slices.Clone(a.cfg.Points.Channels))
	cfg, path := a.cfg, a.cfgPath
	a.mu.Unlock()
	if path != "" {
		saveConfig(path, cfg)
	}
	a.setPoints(func(*PointsState) {})
}

func (a *App) pointsLogins() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]string, len(a.cfg.Points.Channels))
	for i, c := range a.cfg.Points.Channels {
		out[i] = strings.ToLower(c.Login)
	}
	return out
}

func (a *App) loggedInClient() (*gql.Client, error) {
	a.mu.Lock()
	tok := a.token
	a.mu.Unlock()
	if tok == nil {
		return nil, errors.New("not logged in")
	}
	return a.gqlClient(tok), nil
}

// kickPoints makes the points loop re-check now.
func (a *App) kickPoints() {
	a.mu.Lock()
	ch := a.pointsKick
	a.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

// setPoints applies update and lines the channel list up with the config order, keeping
// known state per login.
func (a *App) setPoints(update func(*PointsState)) {
	a.mu.Lock()
	update(&a.points)
	a.points.Enabled = a.cfg.Points.Enabled
	known := map[string]PointsChannelState{}
	for _, c := range a.points.Channels {
		known[strings.ToLower(c.Channel.Login)] = c
	}
	list := make([]PointsChannelState, 0, len(a.cfg.Points.Channels))
	for _, c := range a.cfg.Points.Channels {
		s, ok := known[strings.ToLower(c.Login)]
		if !ok {
			s = PointsChannelState{Channel: models.Channel{Login: c.Login}, State: pointsNotChecked}
		}
		list = append(list, s)
	}
	a.points.Channels = list
	s := a.points
	a.mu.Unlock()
	runtime.EventsEmit(a.ctx, eventPoints, s)
}

// updatePointsChannel changes the state of the channel matching login or channel id.
func (a *App) updatePointsChannel(match func(models.Channel) bool, update func(*PointsChannelState)) {
	a.setPoints(func(s *PointsState) {
		for i := range s.Channels {
			if match(s.Channels[i].Channel) {
				update(&s.Channels[i])
			}
		}
	})
}

func byLogin(login string) func(models.Channel) bool {
	return func(ch models.Channel) bool { return strings.EqualFold(ch.Login, login) }
}

func byID(id string) func(models.Channel) bool {
	return func(ch models.Channel) bool { return ch.ID != "" && ch.ID == id }
}

// startPointsLoop farms channel points until ctx ends: it watches the first live channels of
// the list next to the drops watch, claims bonus chests from PubSub and joins chat read-only.
func (a *App) startPointsLoop(ctx context.Context, tok *auth.Token) {
	kick := make(chan struct{}, 1)
	a.mu.Lock()
	a.pointsKick = kick
	a.mu.Unlock()

	run := &pointsRun{
		client:   a.gqlClient(tok),
		userID:   tok.UserID,
		chat:     &chat.Client{Login: tok.Login, Token: tok.AccessToken},
		checked:  map[string]time.Time{},
		noPoints: map[string]bool{},
		watchers: map[string]context.CancelFunc{},
		cooldown: map[string]time.Time{},
		bets:     map[string]*pendingBet{},
	}
	run.pubsub = &realtime.Client{Token: tok.AccessToken, Handler: func(m realtime.Message) { a.onRealtime(ctx, run, m) }}
	go run.pubsub.Run(ctx)
	go run.chat.Run(ctx)
	a.setPoints(func(s *PointsState) { s.Channels = nil })

	go func() {
		for {
			a.pointsStep(ctx, run)
			select {
			case <-ctx.Done():
				return
			case <-kick:
			case <-time.After(pointsInterval):
			}
		}
	}()
}

func (a *App) pointsStep(ctx context.Context, run *pointsRun) {
	a.mu.Lock()
	cfg := a.cfg.Points
	a.mu.Unlock()
	topics := []string{realtime.PointsTopic(run.userID)} // bonus chests on any watched channel, drops too
	if !cfg.Enabled {
		run.stopAll(nil)
		run.pubsub.SetTopics(topics)
		run.chat.SetChannels(nil)
		a.setPoints(func(s *PointsState) {
			s.Slots = 0
			for i := range s.Channels {
				s.Channels[i].State = pointsNotChecked
			}
		})
		return
	}

	now := time.Now()
	var live []models.Channel
	betting := false
	for _, c := range cfg.Channels {
		ch, err := run.client.StreamInfo(ctx, c.Login)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			a.updatePointsChannel(byLogin(c.Login), func(s *PointsChannelState) { s.Error = err.Error() })
			continue
		}
		if ch.ID == "" {
			a.updatePointsChannel(byLogin(c.Login), func(s *PointsChannelState) { s.Error = "channel not found" })
			continue
		}
		topics = append(topics, realtime.PlaybackTopic(ch.ID))
		if c.Predictions {
			topics = append(topics, realtime.PredictionsTopic(ch.ID))
			betting = true
		}
		live = append(live, ch)
		a.updatePointsChannel(byLogin(c.Login), func(s *PointsChannelState) { s.Channel, s.Error = ch, "" })
		if t, ok := run.checked[strings.ToLower(ch.Login)]; !ok || now.Sub(t) >= balanceInterval {
			run.checked[strings.ToLower(ch.Login)] = now
			a.readBalance(ctx, run, ch, cfg.ClaimBonus)
		}
	}

	w := a.GetWatchState()
	dropsLogin := ""
	if w.State == watchActive || w.State == watchStarting {
		dropsLogin = strings.ToLower(w.Channel.Login)
	}
	logins := make([]string, len(cfg.Channels))
	for i, c := range cfg.Channels {
		logins[i] = c.Login
	}
	slots := points.Slots(dropsLogin != "")
	pick := points.Pick(logins, run.withoutCooldown(run.withPoints(live), now), slots, dropsLogin)

	keep := map[string]bool{}
	for _, ch := range pick {
		keep[strings.ToLower(ch.Login)] = true
	}
	run.stopAll(keep)
	for _, ch := range pick {
		a.startPointsWatch(ctx, run, ch)
	}

	var chatRooms []string
	if cfg.JoinChat {
		for l := range keep {
			chatRooms = append(chatRooms, l)
		}
		if dropsLogin != "" && slices.Contains(logins, dropsLogin) {
			chatRooms = append(chatRooms, dropsLogin)
		}
	}
	run.chat.SetChannels(chatRooms)
	if betting {
		topics = append(topics, realtime.PredictionsUserTopic(run.userID))
	}
	run.pubsub.SetTopics(topics)

	online := map[string]bool{}
	for _, ch := range live {
		online[strings.ToLower(ch.Login)] = ch.Online
	}
	a.setPoints(func(s *PointsState) {
		s.Slots = slots
		for i := range s.Channels {
			l := strings.ToLower(s.Channels[i].Channel.Login)
			switch {
			case keep[l]:
				s.Channels[i].State = pointsWatching
			case l == dropsLogin:
				s.Channels[i].State = pointsDrops
			case run.noPoints[l]:
				s.Channels[i].State = pointsDisabled
			case online[l]:
				s.Channels[i].State = pointsLive
			default:
				s.Channels[i].State = pointsOffline
			}
		}
	})
}

// readBalance loads the balance on ch and claims a bonus chest that is already waiting.
func (a *App) readBalance(ctx context.Context, run *pointsRun, ch models.Channel, claim bool) {
	info, err := run.client.ChannelPoints(ctx, ch.Login)
	if err != nil {
		slog.Warn("Couldn't read channel points on "+ch.DisplayName, "channel", ch.DisplayName, "err", err)
		return
	}
	run.noPoints[strings.ToLower(ch.Login)] = !info.Enabled
	a.updatePointsChannel(byLogin(ch.Login), func(s *PointsChannelState) { s.Balance = info.Balance })
	if claim && info.ClaimID != "" {
		a.claimBonus(ctx, run, info.ChannelID, info.ClaimID)
	}
}

func (a *App) claimBonus(ctx context.Context, run *pointsRun, channelID, claimID string) {
	balance, err := run.client.ClaimPoints(ctx, channelID, claimID)
	name := channelID
	a.mu.Lock()
	for _, c := range a.points.Channels {
		if c.Channel.ID == channelID {
			name = c.Channel.DisplayName
		}
	}
	a.mu.Unlock()
	if err != nil {
		slog.Warn("Couldn't claim the points bonus on "+name, "channel", name, "err", err)
		return
	}
	slog.Info("Points bonus claimed on "+name, "channel", name, "ok", true)
	a.updatePointsChannel(byID(channelID), func(s *PointsChannelState) {
		s.Bonuses++
		if balance > 0 {
			s.Balance = balance
		}
	})
}

// onRealtime handles PubSub events: points earned, bonus chests and streams going up or down.
func (a *App) onRealtime(ctx context.Context, run *pointsRun, m realtime.Message) {
	if p, ok := realtime.ParsePrediction(m); ok {
		a.onPrediction(ctx, run, p)
		return
	}
	if r, ok := realtime.ParsePredictionResult(m); ok {
		a.onPredictionResult(run, r)
		return
	}
	if e, ok := realtime.ParsePoints(m); ok {
		switch e.Type {
		case "points-earned", "points-spent":
			a.updatePointsChannel(byID(e.ChannelID), func(s *PointsChannelState) {
				s.Balance = e.Balance
				if e.Type == "points-earned" {
					s.Earned += e.Earned
				}
			})
		case "claim-available":
			a.mu.Lock()
			claim := a.cfg.Points.ClaimBonus
			a.mu.Unlock()
			if claim {
				go a.claimBonus(ctx, run, e.ChannelID, e.ClaimID)
			}
		}
		return
	}
	if m.TopicName() == realtime.TopicPlayback {
		switch m.Type {
		case "stream-down":
			a.kickPoints()
		case "stream-up":
			time.AfterFunc(streamUpDelay, a.kickPoints)
		}
	}
}

// startPointsWatch watches ch for points unless it is already watched.
func (a *App) startPointsWatch(ctx context.Context, run *pointsRun, ch models.Channel) {
	login := strings.ToLower(ch.Login)
	run.mu.Lock()
	if _, ok := run.watchers[login]; ok {
		run.mu.Unlock()
		return
	}
	wctx, cancel := context.WithCancel(ctx)
	run.watchers[login] = cancel
	run.mu.Unlock()
	slog.Info("Points: watching "+ch.DisplayName, "channel", ch.DisplayName)

	w := &watcher.Watcher{GQL: run.client, UserID: run.userID, UserAgent: run.client.UserAgent}
	go func() {
		err := w.Watch(wctx, watcher.Stream{Channel: ch})
		run.mu.Lock()
		if wctx.Err() == nil { // ended on its own: free the slot, retry the channel after a pause
			delete(run.watchers, login)
			run.cooldown[login] = time.Now()
		}
		run.mu.Unlock()
		cancel()
		switch {
		case errors.Is(err, context.Canceled):
		case errors.Is(err, watcher.ErrOffline):
			a.kickPoints()
		default:
			slog.Warn("Points: watching stopped", "channel", ch.DisplayName, "err", err)
			a.updatePointsChannel(byLogin(login), func(s *PointsChannelState) { s.Error = err.Error() })
			a.kickPoints()
		}
	}()
}

// withoutCooldown drops channels whose watch recently ended on its own (offline before the
// API noticed, or failing), so the loop doesn't restart them on every step.
func (r *pointsRun) withoutCooldown(live []models.Channel, now time.Time) []models.Channel {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []models.Channel
	for _, ch := range live {
		if t, ok := r.cooldown[strings.ToLower(ch.Login)]; ok && now.Sub(t) < watchCooldown {
			continue
		}
		out = append(out, ch)
	}
	return out
}

// withPoints drops channels whose streamer turned channel points off.
func (r *pointsRun) withPoints(live []models.Channel) []models.Channel {
	var out []models.Channel
	for _, ch := range live {
		if !r.noPoints[strings.ToLower(ch.Login)] {
			out = append(out, ch)
		}
	}
	return out
}

// stopAll stops every points watch whose login is not in keep.
func (r *pointsRun) stopAll(keep map[string]bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for login, stop := range r.watchers {
		if !keep[login] {
			stop()
			delete(r.watchers, login)
		}
	}
}
