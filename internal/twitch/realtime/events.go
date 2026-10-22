package realtime

import "encoding/json"

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
