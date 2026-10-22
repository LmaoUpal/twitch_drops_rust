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
