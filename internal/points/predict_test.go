package points

import (
	"strings"
	"testing"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
)

var t0 = time.Date(2026, 10, 22, 12, 0, 0, 0, time.UTC)

func prediction(outcomes ...Outcome) Prediction {
	return Prediction{ID: "e", Status: "ACTIVE", CreatedAt: t0, Window: 2 * time.Minute, Outcomes: outcomes}
}

func bets(strategy string) config.Bets {
	return config.Bets{Strategy: strategy, Percent: 5, MaxPoints: 10000, MinUsers: 20, SecondsBeforeClose: 15}
}

func TestDecideStrategies(t *testing.T) {
	// Crowd clearly on "yes" (80% of voters), but "no" pays more.
	lopsided := prediction(Outcome{ID: "yes", Title: "Yes", Points: 90000, Users: 80}, Outcome{ID: "no", Title: "No", Points: 10000, Users: 20})
	// Close vote: 55% vs 45%.
	close := prediction(Outcome{ID: "a", Points: 30000, Users: 55}, Outcome{ID: "b", Points: 50000, Users: 45})
	now := t0.Add(time.Minute)
	tests := []struct {
		name     string
		p        Prediction
		strategy string
		want     string
	}{
		{"most voted", lopsided, config.StrategyMostVoted, "yes"},
		{"high odds", lopsided, config.StrategyHighOdds, "no"},
		{"smart follows a clear crowd", lopsided, config.StrategySmart, "yes"},
		{"smart takes odds when close", close, config.StrategySmart, "a"},
		{"unknown strategy is smart", close, "", "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := Decide(tt.p, 2000, bets(tt.strategy), now)
			if err != nil || b.OutcomeID != tt.want || b.Points != 100 {
				t.Fatalf("got %+v, %v", b, err)
			}
		})
	}
}

func TestDecideLimitsAndSkips(t *testing.T) {
	p := prediction(Outcome{ID: "a", Points: 100, Users: 15}, Outcome{ID: "b", Points: 100, Users: 10})
	now := t0.Add(time.Minute)

	if b, err := Decide(p, 1_000_000, bets(""), now); err != nil || b.Points != 10000 {
		t.Fatalf("capped at max points: %+v %v", b, err)
	}
	tests := []struct {
		name    string
		p       Prediction
		balance int
		now     time.Time
		want    string
	}{
		{"too few voters", prediction(Outcome{ID: "a", Users: 5}, Outcome{ID: "b", Users: 5}), 2000, now, "voters"},
		{"bet below minimum", p, 100, now, "minimum"},
		{"window closed", p, 2000, t0.Add(3 * time.Minute), "closed"},
		{"locked", func() Prediction { q := p; q.Status = "LOCKED"; return q }(), 2000, now, "closed"},
		{"single outcome", prediction(Outcome{ID: "a", Users: 50}), 2000, now, "outcomes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Decide(tt.p, tt.balance, bets(""), tt.now); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want error with %q, got %v", tt.want, err)
			}
		})
	}
}

func TestHighOddsIgnoresEmptyOutcomes(t *testing.T) {
	p := prediction(Outcome{ID: "a", Points: 500, Users: 30}, Outcome{ID: "b", Points: 0, Users: 0}, Outcome{ID: "c", Points: 900, Users: 10})
	if b, _ := Decide(p, 2000, bets(config.StrategyHighOdds), t0); b.OutcomeID != "a" {
		t.Fatalf("got %+v", b)
	}
}

func TestTimingAndPayout(t *testing.T) {
	p := prediction(Outcome{ID: "a", Points: 300}, Outcome{ID: "b", Points: 100})
	if !p.BetAt(15).Equal(t0.Add(105 * time.Second)) {
		t.Fatalf("bet at %v", p.BetAt(15))
	}
	if got := p.Payout("b", 100); got != 400 {
		t.Fatalf("payout %d", got)
	}
}
