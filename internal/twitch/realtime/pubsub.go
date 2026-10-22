package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DefaultURL is the Twitch PubSub endpoint (still serving the TV client, checked 2026-10-21).
const DefaultURL = "wss://pubsub-edge.twitch.tv/v1"

// MaxTopics is how many topics Twitch allows on one connection; extra topics are not listened to.
const MaxTopics = 50

// Message is one PubSub event.
type Message struct {
	Topic   string          // full topic, e.g. "community-points-user-v1.123"
	Type    string          // event type inside the message, e.g. "points-earned"
	Payload json.RawMessage // the event JSON (Twitch sends it as a string inside the frame)
}

// TopicName is the topic without its id ("raid" for "raid.123").
func (m Message) TopicName() string { name, _, _ := strings.Cut(m.Topic, "."); return name }

// TopicID is the user or channel id the topic is about ("123" for "raid.123").
func (m Message) TopicID() string { _, id, _ := strings.Cut(m.Topic, "."); return id }

// Client keeps one PubSub connection open while Run runs. Topics can change at any time.
type Client struct {
	URL     string // DefaultURL when empty
	Token   string // OAuth token, sent in LISTEN requests only
	Handler func(Message)
	// PingEvery defaults to 4 minutes; Twitch drops connections that don't PING within 5.
	PingEvery time.Duration
	Dialer    *websocket.Dialer
	Log       *slog.Logger

	mu     sync.Mutex
	topics []string
	conn   *websocket.Conn
}

// SetTopics replaces the topics listened to. On a live connection only the difference is
// sent; the full list is listened to again after each reconnect.
func (c *Client) SetTopics(topics []string) {
	topics = slices.Compact(slices.Sorted(slices.Values(topics)))
	if len(topics) > MaxTopics {
		c.logger().Warn("Too many realtime topics, some are ignored", "tag", "PubSub", "topics", len(topics))
		topics = topics[:MaxTopics]
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var add, remove []string
	for _, t := range topics {
		if !slices.Contains(c.topics, t) {
			add = append(add, t)
		}
	}
	for _, t := range c.topics {
		if !slices.Contains(topics, t) {
			remove = append(remove, t)
		}
	}
	c.topics = topics
	if c.conn == nil {
		return
	}
	if len(remove) > 0 {
		c.writeLocked("UNLISTEN", remove)
	}
	if len(add) > 0 {
		c.writeLocked("LISTEN", add)
	}
}

// Run keeps the connection up until ctx ends, reconnecting with backoff.
func (c *Client) Run(ctx context.Context) {
	backoff := time.Second
	for ctx.Err() == nil {
		started := time.Now()
		err := c.session(ctx)
		if ctx.Err() != nil {
			return
		}
		if time.Since(started) > time.Minute {
			backoff = time.Second // the connection was healthy; reconnect quickly
		}
		c.logger().Debug("Realtime connection lost, reconnecting", "tag", "PubSub", "err", err, "in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 2*time.Minute)
	}
}

// session runs one connection until it fails or ctx ends.
func (c *Client) session(ctx context.Context) error {
	url := c.URL
	if url == "" {
		url = DefaultURL
	}
	dialer := c.Dialer
	if dialer == nil {
		dialer = websocket.DefaultDialer
	}
	conn, _, err := dialer.DialContext(ctx, url, nil)
	if err != nil {
		return err
	}
	defer conn.Close()

	c.mu.Lock()
	c.conn = conn
	if len(c.topics) > 0 {
		c.writeLocked("LISTEN", c.topics)
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()
	}()

	done := make(chan struct{})
	defer close(done)
	go func() {
		ping := time.NewTicker(c.pingEvery())
		defer ping.Stop()
		for {
			select {
			case <-ctx.Done():
				conn.Close()
				return
			case <-done:
				return
			case <-ping.C:
				c.mu.Lock()
				err := conn.WriteJSON(map[string]string{"type": "PING"})
				c.mu.Unlock()
				if err != nil {
					conn.Close()
					return
				}
			}
		}
	}()

	for {
		// Every ping gets a PONG, so a silent connection is a dead one.
		conn.SetReadDeadline(time.Now().Add(c.pingEvery() + 30*time.Second))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		if err := c.handle(data); err != nil {
			return err
		}
	}
}

type frame struct {
	Type  string `json:"type"`
	Error string `json:"error"`
	Data  struct {
		Topic   string `json:"topic"`
		Message string `json:"message"`
	} `json:"data"`
}

// handle processes one frame; an error means the connection must be re-established.
func (c *Client) handle(data []byte) error {
	var f frame
	if err := json.Unmarshal(data, &f); err != nil {
		return nil // not ours to understand; keep the connection
	}
	switch f.Type {
	case "RECONNECT":
		return errors.New("pubsub: server asked to reconnect")
	case "RESPONSE":
		if f.Error != "" {
			c.logger().Warn("Realtime listen failed: "+f.Error, "tag", "PubSub")
		}
	case "MESSAGE":
		var inner struct {
			Type string `json:"type"`
		}
		json.Unmarshal([]byte(f.Data.Message), &inner)
		if c.Handler != nil {
			c.Handler(Message{Topic: f.Data.Topic, Type: inner.Type, Payload: json.RawMessage(f.Data.Message)})
		}
	}
	return nil
}

// writeLocked sends a LISTEN or UNLISTEN request; c.mu must be held.
func (c *Client) writeLocked(kind string, topics []string) {
	req := map[string]any{
		"type":  kind,
		"nonce": nonce(),
		"data":  map[string]any{"topics": topics, "auth_token": c.Token},
	}
	if err := c.conn.WriteJSON(req); err != nil {
		c.conn.Close() // the read loop notices and reconnects
	}
}

func (c *Client) pingEvery() time.Duration {
	if c.PingEvery > 0 {
		return c.PingEvery
	}
	return 4 * time.Minute
}

func (c *Client) logger() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.Default()
}

func nonce() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
