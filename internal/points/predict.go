package points

import (
	"fmt"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
)

// MinBet is the smallest bet Twitch accepts.
const MinBet = 10

// smartGap is the vote share difference (percentage points) above which the smart strategy
// trusts the crowd; closer predictions go for the better payout instead.
const smartGap = 20

type Outcome struct {
	ID     string
	Title  string
	Points int // points bet on this outcome so far
	Users  int // users who bet on it
}

// Prediction is a channel points prediction as PubSub reports it.
type Prediction struct {
	ID        string
	ChannelID string
	Title     string
	Status    string // ACTIVE, LOCKED, RESOLVE_PENDING, RESOLVED, CANCELED
	CreatedAt time.Time
	Window    time.Duration // how long bets are open after CreatedAt
	Outcomes  []Outcome
	Winner    string // winning outcome id once resolved
}

// ClosesAt is when betting closes.
func (p Prediction) ClosesAt() time.Time { return p.CreatedAt.Add(p.Window) }

// BetAt is when to place the bet: secondsBeforeClose before betting closes, so the votes
// so far show where the crowd is.
func (p Prediction) BetAt(secondsBeforeClose int) time.Time {
	return p.ClosesAt().Add(-time.Duration(secondsBeforeClose) * time.Second)
}

func (p Prediction) totals() (points, users int) {
	for _, o := range p.Outcomes {
		points += o.Points
		users += o.Users
	}
	return
}

// Bet is a decision to bet Points on outcome OutcomeID.
type Bet struct {
	OutcomeID string
	Outcome   string
	Points    int
}

// Decide picks the outcome and amount for a prediction with the given balance, or returns
// why it should be skipped.
func Decide(p Prediction, balance int, s config.Bets, now time.Time) (Bet, error) {
	if p.Status != "ACTIVE" || !now.Before(p.ClosesAt()) {
		return Bet{}, fmt.Errorf("betting is closed")
	}
	if len(p.Outcomes) < 2 {
		return Bet{}, fmt.Errorf("prediction has %d outcomes", len(p.Outcomes))
	}
	if _, users := p.totals(); users < s.MinUsers {
		return Bet{}, fmt.Errorf("only %d voters (minimum %d)", users, s.MinUsers)
	}
	amount := balance * s.Percent / 100
	if s.MaxPoints > 0 {
		amount = min(amount, s.MaxPoints)
	}
	if amount < MinBet {
		return Bet{}, fmt.Errorf("bet of %d is below the minimum %d (balance %d)", amount, MinBet, balance)
	}
	o := choose(p, s.Strategy)
	return Bet{OutcomeID: o.ID, Outcome: o.Title, Points: amount}, nil
}

func choose(p Prediction, strategy string) Outcome {
	switch strategy {
	case config.StrategyMostVoted:
		return mostVoted(p)
	case config.StrategyHighOdds:
		return highOdds(p)
	default: // smart
		total := 0
		first, second := 0, 0
		for _, o := range p.Outcomes {
			total += o.Users
			switch {
			case o.Users > first:
				first, second = o.Users, first
			case o.Users > second:
				second = o.Users
			}
		}
		if total > 0 && (first-second)*100/total > smartGap {
			return mostVoted(p)
		}
		return highOdds(p)
	}
}

func mostVoted(p Prediction) Outcome {
	best := p.Outcomes[0]
	for _, o := range p.Outcomes[1:] {
		if o.Users > best.Users || (o.Users == best.Users && o.Points > best.Points) {
			best = o
		}
	}
	return best
}

// highOdds picks the outcome paying the most per point bet: the one with the fewest points
// on it. Outcomes nobody bet on are left out, their payout says nothing.
func highOdds(p Prediction) Outcome {
	var best *Outcome
	for i, o := range p.Outcomes {
		if o.Points == 0 {
			continue
		}
		if best == nil || o.Points < best.Points {
			best = &p.Outcomes[i]
		}
	}
	if best == nil {
		return mostVoted(p)
	}
	return *best
}

// Payout is what a bet of points on outcome would return, stake included, if it wins at
// the current pool.
func (p Prediction) Payout(outcomeID string, points int) int {
	total, _ := p.totals()
	for _, o := range p.Outcomes {
		if o.ID == outcomeID && o.Points > 0 {
			return points * total / o.Points
		}
	}
	return 0
}
