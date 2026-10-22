package realtime

import (
	"encoding/json"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/points"
)

// Topic names used by the app.
const (
	TopicPoints   = "community-points-user-v1" // .<user id>: points earned/spent, bonus claim available
	TopicPlayback = "video-playback-by-id"     // .<channel id>: stream-up, stream-down, viewcount
)

// PointsTopic is the topic with the user's channel points events on every channel.
func PointsTopic(userID string) string { return TopicPoints + "." + userID }

// PlaybackTopic is the topic with a channel's stream up/down events.
func PlaybackTopic(channelID string) string { return TopicPlayback + "." + channelID }

// PointsEvent is a parsed community-points-user-v1 message.
type PointsEvent struct {
	Type      string // "points-earned", "points-spent" or "claim-available"
	ChannelID string
	Earned    int    // points-earned: points gained
	Reason    string // points-earned: "WATCH", "CLAIM", "WATCH_STREAK", ...
	Balance   int    // points-earned/spent: balance after the change
	ClaimID   string // claim-available: the bonus to claim
}

// ParsePoints reads a community-points-user-v1 message; ok is false for other topics or
// event types the app doesn't use.
func ParsePoints(m Message) (PointsEvent, bool) {
	if m.TopicName() != TopicPoints {
		return PointsEvent{}, false
	}
	var p struct {
		Data struct {
			ChannelID string `json:"channel_id"`
			PointGain struct {
				ChannelID   string `json:"channel_id"`
				TotalPoints int    `json:"total_points"`
				ReasonCode  string `json:"reason_code"`
			} `json:"point_gain"`
			Balance struct {
				ChannelID string `json:"channel_id"`
				Balance   int    `json:"balance"`
			} `json:"balance"`
			Claim struct {
				ID        string `json:"id"`
				ChannelID string `json:"channel_id"`
			} `json:"claim"`
		} `json:"data"`
	}
	if err := json.Unmarshal(m.Payload, &p); err != nil {
		return PointsEvent{}, false
	}
	d := p.Data
	e := PointsEvent{Type: m.Type}
	switch m.Type {
	case "points-earned", "points-spent":
		e.ChannelID, e.Balance = first(d.Balance.ChannelID, d.ChannelID, d.PointGain.ChannelID), d.Balance.Balance
		e.Earned, e.Reason = d.PointGain.TotalPoints, d.PointGain.ReasonCode
	case "claim-available":
		e.ChannelID, e.ClaimID = first(d.Claim.ChannelID, d.ChannelID), d.Claim.ID
	default:
		return PointsEvent{}, false
	}
	return e, e.ChannelID != ""
}

func first(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// Prediction topics.
const (
	TopicPredictions     = "predictions-channel-v1" // .<channel id>: event-created, event-updated
	TopicPredictionsUser = "predictions-user-v1"    // .<user id>: prediction-made, prediction-result
)

// PredictionsTopic is the topic with a channel's predictions.
func PredictionsTopic(channelID string) string { return TopicPredictions + "." + channelID }

// PredictionsUserTopic is the topic with the results of the user's own bets.
func PredictionsUserTopic(userID string) string { return TopicPredictionsUser + "." + userID }

// ParsePrediction reads an event-created or event-updated message of predictions-channel-v1.
func ParsePrediction(m Message) (points.Prediction, bool) {
	if m.TopicName() != TopicPredictions || (m.Type != "event-created" && m.Type != "event-updated") {
		return points.Prediction{}, false
	}
	var p struct {
		Data struct {
			Event struct {
				ID                      string    `json:"id"`
				ChannelID               string    `json:"channel_id"`
				Title                   string    `json:"title"`
				Status                  string    `json:"status"`
				CreatedAt               time.Time `json:"created_at"`
				PredictionWindowSeconds float64   `json:"prediction_window_seconds"`
				WinningOutcomeID        string    `json:"winning_outcome_id"`
				Outcomes                []struct {
					ID          string `json:"id"`
					Title       string `json:"title"`
					TotalPoints int    `json:"total_points"`
					TotalUsers  int    `json:"total_users"`
				} `json:"outcomes"`
			} `json:"event"`
		} `json:"data"`
	}
	if err := json.Unmarshal(m.Payload, &p); err != nil || p.Data.Event.ID == "" {
		return points.Prediction{}, false
	}
	e := p.Data.Event
	out := points.Prediction{
		ID: e.ID, ChannelID: first(e.ChannelID, m.TopicID()), Title: e.Title, Status: e.Status,
		CreatedAt: e.CreatedAt, Window: time.Duration(e.PredictionWindowSeconds * float64(time.Second)),
		Winner: e.WinningOutcomeID,
	}
	for _, o := range e.Outcomes {
		out.Outcomes = append(out.Outcomes, points.Outcome{ID: o.ID, Title: o.Title, Points: o.TotalPoints, Users: o.TotalUsers})
	}
	return out, true
}

// PredictionResult is the outcome of one of the user's bets.
type PredictionResult struct {
	EventID   string
	ChannelID string
	Type      string // "WIN", "LOSE" or "REFUND"
	PointsWon int    // payout including the stake; 0 when lost
}

// ParsePredictionResult reads a prediction-result message of predictions-user-v1.
func ParsePredictionResult(m Message) (PredictionResult, bool) {
	if m.TopicName() != TopicPredictionsUser || m.Type != "prediction-result" {
		return PredictionResult{}, false
	}
	var p struct {
		Data struct {
			Prediction struct {
				EventID   string `json:"event_id"`
				ChannelID string `json:"channel_id"`
				Result    *struct {
					Type      string `json:"type"`
					PointsWon int    `json:"points_won"`
				} `json:"result"`
			} `json:"prediction"`
		} `json:"data"`
	}
	if err := json.Unmarshal(m.Payload, &p); err != nil || p.Data.Prediction.Result == nil {
		return PredictionResult{}, false
	}
	pr := p.Data.Prediction
	return PredictionResult{EventID: pr.EventID, ChannelID: pr.ChannelID, Type: pr.Result.Type, PointsWon: pr.Result.PointsWon}, true
}

// TopicRaid is the topic with a channel's raids: raid_update_v2 when one is announced,
// raid_go_v2 when it goes, raid_cancel_v2.
const TopicRaid = "raid"

// RaidTopic is the topic with the raids a channel starts.
func RaidTopic(channelID string) string { return TopicRaid + "." + channelID }

// Raid is a raid one channel starts on another.
type Raid struct {
	ID          string
	SourceID    string // the raiding channel
	TargetID    string
	TargetLogin string
	TargetName  string
}

// ParseRaid reads a raid message; Message.Type tells announced, gone or cancelled apart.
func ParseRaid(m Message) (Raid, bool) {
	if m.TopicName() != TopicRaid {
		return Raid{}, false
	}
	var p struct {
		Raid struct {
			ID                string `json:"id"`
			SourceID          string `json:"source_id"`
			TargetID          string `json:"target_id"`
			TargetLogin       string `json:"target_login"`
			TargetDisplayName string `json:"target_display_name"`
		} `json:"raid"`
	}
	if err := json.Unmarshal(m.Payload, &p); err != nil || p.Raid.ID == "" {
		return Raid{}, false
	}
	r := p.Raid
	return Raid{ID: r.ID, SourceID: first(r.SourceID, m.TopicID()), TargetID: r.TargetID, TargetLogin: r.TargetLogin, TargetName: first(r.TargetDisplayName, r.TargetLogin)}, true
}
