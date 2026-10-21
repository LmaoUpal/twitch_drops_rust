package main

import (
	"fmt"
	"time"

	"github.com/gorilla/websocket"

	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/gql"
)

// runPubSub listens to Twitch PubSub topics and prints every frame for d. The token is sent
// in the LISTEN request only and never printed.
func runPubSub(topics []string, d time.Duration) error {
	_, tok, err := client()
	if err != nil {
		return err
	}
	ws, _, err := websocket.DefaultDialer.Dial("wss://pubsub-edge.twitch.tv/v1", nil)
	if err != nil {
		return err
	}
	defer ws.Close()
	listen := map[string]any{
		"type":  "LISTEN",
		"nonce": gql.NewSessionID(),
		"data":  map[string]any{"topics": topics, "auth_token": tok.AccessToken},
	}
	if err := ws.WriteJSON(listen); err != nil {
		return err
	}
	fmt.Printf("listening to %v for %s\n", topics, d)
	time.AfterFunc(d, func() { ws.Close() })
	go func() {
		for range time.Tick(time.Minute) {
			if ws.WriteJSON(map[string]string{"type": "PING"}) != nil {
				return
			}
		}
	}()
	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			return nil // closed after d, or by Twitch
		}
		fmt.Printf("%s %s\n", time.Now().Format("15:04:05"), msg)
	}
}
