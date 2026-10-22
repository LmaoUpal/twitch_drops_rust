package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
	"github.com/LmaoUpal/twitch_drops_rust/internal/models"
	"github.com/LmaoUpal/twitch_drops_rust/internal/points"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/gql"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/realtime"
)

// pendingBet is a prediction on a betting channel: scheduled until its bet time, then placed
// (or skipped) and kept until Twitch reports the result.
type pendingBet struct {
	pred  points.Prediction
	login string
	bet   *points.Bet
	timer *time.Timer
}

// SetPointsChannelPredictions turns automatic betting on a channel's predictions on or off.
func (a *App) SetPointsChannelPredictions(login string, on bool) {
	a.editPointsChannels(func(list []config.PointsChannel) []config.PointsChannel {
		for i := range list {
			if strings.EqualFold(list[i].Login, login) {
				list[i].Predictions = on
			}
		}
		return list
	})
	a.kickPoints() // subscribes to or drops the channel's predictions topic
}

// onPrediction tracks predictions of betting channels and schedules each bet shortly before
// betting closes.
func (a *App) onPrediction(ctx context.Context, run *pointsRun, p points.Prediction) {
	ch, ok := a.pointsChannelByID(p.ChannelID)
	if !ok {
		return
	}
	a.mu.Lock()
	secs := a.cfg.Points.Bets.SecondsBeforeClose
	a.mu.Unlock()

	run.mu.Lock()
	defer run.mu.Unlock()
	pb, known := run.bets[p.ID]
	switch {
	case !known && p.Status == "ACTIVE":
		run.pruneBets(time.Now())
		pb = &pendingBet{pred: p, login: ch.Login}
		run.bets[p.ID] = pb
		delay := max(time.Until(p.BetAt(secs)), 0)
		pb.timer = time.AfterFunc(delay, func() { a.placeBet(ctx, run, p.ID) })
		slog.Info(fmt.Sprintf("Prediction on %s: %q — deciding in %ds", ch.DisplayName, p.Title, int(delay.Seconds())), "channel", ch.DisplayName)
	case known:
		if p.CreatedAt.IsZero() {
			p.CreatedAt, p.Window = pb.pred.CreatedAt, pb.pred.Window
		}
		pb.pred = p
		if (p.Status == "RESOLVED" || p.Status == "CANCELED") && pb.bet == nil {
			pb.timer.Stop()
			delete(run.bets, p.ID)
		}
	}
}

// placeBet decides and places the bet for prediction id with the current balance.
func (a *App) placeBet(ctx context.Context, run *pointsRun, id string) {
	run.mu.Lock()
	pb, ok := run.bets[id]
	if !ok {
		run.mu.Unlock()
		return
	}
	p, login := pb.pred, pb.login
	run.mu.Unlock()

	a.mu.Lock()
	cfg := a.cfg.Points
	a.mu.Unlock()
	i := slices.IndexFunc(cfg.Channels, func(c config.PointsChannel) bool { return strings.EqualFold(c.Login, login) })
	if !cfg.Enabled || i < 0 || !cfg.Channels[i].Predictions {
		return
	}
	name := a.pointsChannelName(login)
	info, err := run.client.ChannelPoints(ctx, login)
	if err != nil {
		slog.Warn("Couldn't read the balance for a prediction on "+name, "channel", name, "err", err)
		return
	}
	bet, err := points.Decide(p, info.Balance, cfg.Bets, time.Now())
	if err != nil {
		slog.Info(fmt.Sprintf("Skipped prediction %q on %s: %v", p.Title, name, err), "channel", name)
		return
	}
	err = run.client.MakePrediction(ctx, p.ID, bet.OutcomeID, bet.Points)
	switch {
	case errors.Is(err, gql.ErrMustAcceptTOS):
		slog.Warn("Can't bet: Twitch wants the predictions terms accepted. Make one prediction by hand on twitch.tv, then bets work.", "channel", name)
		a.setPoints(func(s *PointsState) { s.BetsNeedTOS = true })
		return
	case err != nil:
		slog.Warn("Bet failed on "+name, "channel", name, "err", err)
		return
	}
	run.mu.Lock()
	pb.bet = &bet
	run.mu.Unlock()
	slog.Info(fmt.Sprintf("Bet %d on %q — %q (%s), pays ~%d if it wins", bet.Points, bet.Outcome, p.Title, name, p.Payout(bet.OutcomeID, bet.Points)), "channel", name, "ok", true)
	a.setPoints(func(s *PointsState) { s.BetsNeedTOS = false })
	a.updatePointsChannel(byLogin(login), func(s *PointsChannelState) { s.Bets++ })
}

// onPredictionResult reports how a bet placed by the app ended.
func (a *App) onPredictionResult(run *pointsRun, r realtime.PredictionResult) {
	run.mu.Lock()
	pb, ok := run.bets[r.EventID]
	delete(run.bets, r.EventID)
	run.mu.Unlock()
	if !ok || pb.bet == nil {
		return // not a bet the app placed
	}
	name := a.pointsChannelName(pb.login)
	profit := 0
	switch r.Type {
	case "WIN":
		profit = r.PointsWon - pb.bet.Points
		slog.Info(fmt.Sprintf("Prediction won on %s: +%d (%q)", name, profit, pb.pred.Title), "channel", name, "ok", true)
	case "LOSE":
		profit = -pb.bet.Points
		slog.Info(fmt.Sprintf("Prediction lost on %s: -%d (%q)", name, pb.bet.Points, pb.pred.Title), "channel", name)
	default:
		slog.Info(fmt.Sprintf("Prediction refunded on %s (%q)", name, pb.pred.Title), "channel", name)
	}
	a.updatePointsChannel(byLogin(pb.login), func(s *PointsChannelState) { s.BetProfit += profit })
}

func (a *App) pointsChannelByID(id string) (models.Channel, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, c := range a.points.Channels {
		if c.Channel.ID == id {
			return c.Channel, true
		}
	}
	return models.Channel{}, false
}

func (a *App) pointsChannelName(login string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, c := range a.points.Channels {
		if strings.EqualFold(c.Channel.Login, login) && c.Channel.DisplayName != "" {
			return c.Channel.DisplayName
		}
	}
	return login
}

// pruneBets drops predictions that closed long ago without a result reaching the app
// (unsubscribed channel, missed event); run.mu must be held. Predictions are kept until
// resolved so later vote updates don't schedule a second decision.
func (r *pointsRun) pruneBets(now time.Time) {
	for id, pb := range r.bets {
		if now.Sub(pb.pred.ClosesAt()) > 6*time.Hour {
			pb.timer.Stop()
			delete(r.bets, id)
		}
	}
}
