package realtime

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

type request struct {
	Type string `json:"type"`
	Data struct {
		Topics    []string `json:"topics"`
		AuthToken string   `json:"auth_token"`
	} `json:"data"`
}

// earned is a real points-earned message (2026-10-21), as Twitch sends it: JSON in a string.
const earned = `{"type":"points-earned","data":{"timestamp":"2026-10-21T18:50:13Z","channel_id":"459331509","point_gain":{"user_id":"1020370763","channel_id":"459331509","total_points":10,"baseline_points":10,"reason_code":"WATCH","multipliers":[]},"balance":{"user_id":"1020370763","channel_id":"459331509","balance":10}}}`

func messageFrame(topic, msg string) []byte {
	b, _ := json.Marshal(map[string]any{"type": "MESSAGE", "data": map[string]string{"topic": topic, "message": msg}})
	return b
}

func TestClientListensReceivesAndRelistensAfterReconnect(t *testing.T) {
	requests := make(chan request, 10)
	conns := make(chan *websocket.Conn, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conns <- conn
		for {
			var req request
			if err := conn.ReadJSON(&req); err != nil {
				return
			}
			requests <- req
		}
	}))
	defer srv.Close()

	got := make(chan Message, 10)
	c := &Client{URL: "ws" + strings.TrimPrefix(srv.URL, "http"), Token: "tok", Handler: func(m Message) { got <- m }}
	c.SetTopics([]string{PointsTopic("1"), PlaybackTopic("2")})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)

	first := <-conns
	req := next(t, requests)
	if req.Type != "LISTEN" || req.Data.AuthToken != "tok" || strings.Join(req.Data.Topics, ",") != "community-points-user-v1.1,video-playback-by-id.2" {
		t.Fatalf("first listen: %+v", req)
	}

	first.WriteMessage(websocket.TextMessage, messageFrame("community-points-user-v1.1", earned))
	var m Message
	select {
	case m = <-got:
	case <-time.After(2 * time.Second):
		t.Fatal("no message delivered")
	}
	e, ok := ParsePoints(m)
	if !ok || e.Type != "points-earned" || e.ChannelID != "459331509" || e.Earned != 10 || e.Reason != "WATCH" || e.Balance != 10 {
		t.Fatalf("parsed %+v (ok %v) from %+v", e, ok, m)
	}

	c.SetTopics([]string{PointsTopic("1"), PlaybackTopic("3")})
	unlisten, listen := next(t, requests), next(t, requests)
	if unlisten.Type != "UNLISTEN" || unlisten.Data.Topics[0] != "video-playback-by-id.2" || listen.Type != "LISTEN" || listen.Data.Topics[0] != "video-playback-by-id.3" {
		t.Fatalf("diff: %+v then %+v", unlisten, listen)
	}

	first.WriteMessage(websocket.TextMessage, []byte(`{"type":"RECONNECT"}`))
	<-conns
	req = next(t, requests)
	if req.Type != "LISTEN" || strings.Join(req.Data.Topics, ",") != "community-points-user-v1.1,video-playback-by-id.3" {
		t.Fatalf("listen after reconnect: %+v", req)
	}
}

func next(t *testing.T, ch chan request) request {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("no request")
		return request{}
	}
}

func TestParsePointsClaim(t *testing.T) {
	m := Message{Topic: "community-points-user-v1.1", Type: "claim-available",
		Payload: json.RawMessage(`{"type":"claim-available","data":{"timestamp":"x","claim":{"id":"c-1","user_id":"1","channel_id":"42"}}}`)}
	e, ok := ParsePoints(m)
	if !ok || e.ChannelID != "42" || e.ClaimID != "c-1" {
		t.Fatalf("got %+v %v", e, ok)
	}
	if _, ok := ParsePoints(Message{Topic: "raid.1", Type: "raid_update_v2"}); ok {
		t.Fatal("other topics are not points events")
	}
}

func TestParsePrediction(t *testing.T) {
	msg := `{"type":"event-created","data":{"timestamp":"2026-10-22T12:00:01Z","event":{"id":"ev-1","channel_id":"42","created_at":"2026-10-22T12:00:00Z","ended_at":null,"locked_at":null,"outcomes":[{"id":"o1","color":"BLUE","title":"Yes","total_points":1500,"total_users":12,"top_predictors":[],"badge":{}},{"id":"o2","color":"PINK","title":"No","total_points":500,"total_users":3,"top_predictors":[],"badge":{}}],"prediction_window_seconds":120,"status":"ACTIVE","title":"Win?","winning_outcome_id":null}}}`
	p, ok := ParsePrediction(Message{Topic: "predictions-channel-v1.42", Type: "event-created", Payload: json.RawMessage(msg)})
	if !ok || p.ID != "ev-1" || p.ChannelID != "42" || p.Status != "ACTIVE" || p.Window != 2*time.Minute || len(p.Outcomes) != 2 {
		t.Fatalf("got %+v %v", p, ok)
	}
	if o := p.Outcomes[1]; o.ID != "o2" || o.Title != "No" || o.Points != 500 || o.Users != 3 {
		t.Fatalf("outcome %+v", o)
	}
	if !p.ClosesAt().Equal(time.Date(2026, 10, 22, 12, 2, 0, 0, time.UTC)) {
		t.Fatalf("closes at %v", p.ClosesAt())
	}

	res := `{"type":"prediction-result","data":{"timestamp":"x","prediction":{"id":"p","event_id":"ev-1","outcome_id":"o2","channel_id":"42","points":100,"predicted_at":"x","updated_at":"x","user_id":"1","result":{"type":"WIN","points_won":400,"is_acknowledged":false},"user_display_name":null}}}`
	r, ok := ParsePredictionResult(Message{Topic: "predictions-user-v1.1", Type: "prediction-result", Payload: json.RawMessage(res)})
	if !ok || r.EventID != "ev-1" || r.ChannelID != "42" || r.Type != "WIN" || r.PointsWon != 400 {
		t.Fatalf("result %+v %v", r, ok)
	}
}
