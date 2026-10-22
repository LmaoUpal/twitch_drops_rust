package chat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestClientJoinsAnswersPingAndNeverSpeaks(t *testing.T) {
	lines := make(chan string, 20)
	conns := make(chan *websocket.Conn, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		conns <- conn
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			for _, l := range strings.Split(strings.TrimSpace(string(data)), "\r\n") {
				lines <- l
			}
		}
	}))
	defer srv.Close()

	c := &Client{URL: "ws" + strings.TrimPrefix(srv.URL, "http"), Login: "Me", Token: "tok"}
	c.SetChannels([]string{"B", "a"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go c.Run(ctx)
	conn := <-conns

	for _, want := range []string{"PASS oauth:tok", "NICK me", "JOIN #a,#b"} {
		if got := nextLine(t, lines); got != want {
			t.Fatalf("want %q, got %q", want, got)
		}
	}
	conn.WriteMessage(websocket.TextMessage, []byte("PING :tmi.twitch.tv\r\n"))
	if got := nextLine(t, lines); got != "PONG :tmi.twitch.tv" {
		t.Fatalf("pong: %q", got)
	}
	c.SetChannels([]string{"a", "c"})
	if p, j := nextLine(t, lines), nextLine(t, lines); p != "PART #b" || j != "JOIN #c" {
		t.Fatalf("diff: %q, %q", p, j)
	}
	cancel()
	time.Sleep(100 * time.Millisecond)
	close(lines)
	for l := range lines {
		if strings.HasPrefix(l, "PRIVMSG") {
			t.Fatalf("client must never send messages: %q", l)
		}
	}
}

func nextLine(t *testing.T, ch chan string) string {
	t.Helper()
	select {
	case l := <-ch:
		return l
	case <-time.After(5 * time.Second):
		t.Fatal("no line")
		return ""
	}
}
