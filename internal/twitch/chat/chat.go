// Package chat joins Twitch chat rooms read-only over IRC WebSocket, so the account shows up
// in the channels' chatter lists while watching. It never sends chat messages: the only
// commands it writes are login, JOIN/PART and PONG.
package chat

import (
	"context"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// DefaultURL is Twitch's IRC over WebSocket endpoint.
const DefaultURL = "wss://irc-ws.chat.twitch.tv:443"

// Client keeps one IRC connection joined to Channels while Run runs.
type Client struct {
	URL    string // DefaultURL when empty
	Login  string // the account's login, sent as NICK
	Token  string // OAuth token with chat:read, sent as PASS
	Dialer *websocket.Dialer
	Log    *slog.Logger

	mu       sync.Mutex
	channels []string
	conn     *websocket.Conn
}

// SetChannels replaces the joined channels (logins); on a live connection only the
// difference is sent.
func (c *Client) SetChannels(logins []string) {
	logins = slices.Clone(logins)
	for i, l := range logins {
		logins[i] = strings.ToLower(l)
	}
	logins = slices.Compact(slices.Sorted(slices.Values(logins)))
	c.mu.Lock()
	defer c.mu.Unlock()
	var join, part []string
	for _, l := range logins {
		if !slices.Contains(c.channels, l) {
			join = append(join, l)
		}
	}
	for _, l := range c.channels {
		if !slices.Contains(logins, l) {
			part = append(part, l)
		}
	}
	c.channels = logins
	if c.conn == nil {
		return
	}
	if len(part) > 0 {
		c.sendLocked("PART " + rooms(part))
	}
	if len(join) > 0 {
		c.sendLocked("JOIN " + rooms(join))
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
			backoff = time.Second
		}
		c.logger().Debug("Chat connection lost, reconnecting", "tag", "Chat", "err", err, "in", backoff)
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, 2*time.Minute)
	}
}

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
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()

	c.mu.Lock()
	c.conn = conn
	c.sendLocked("PASS oauth:" + c.Token)
	c.sendLocked("NICK " + strings.ToLower(c.Login))
	if len(c.channels) > 0 {
		c.sendLocked("JOIN " + rooms(c.channels))
	}
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		c.conn = nil
		c.mu.Unlock()
	}()

	for {
		// Twitch PINGs about every 5 minutes; silence for longer means the connection is dead.
		conn.SetReadDeadline(time.Now().Add(7 * time.Minute))
		_, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\r\n") {
			switch {
			case strings.HasPrefix(line, "PING "):
				c.mu.Lock()
				c.sendLocked("PONG " + strings.TrimPrefix(line, "PING "))
				c.mu.Unlock()
			case strings.Contains(line, " NOTICE * :Login authentication failed"):
				c.logger().Warn("Chat login failed", "tag", "Chat")
			case strings.HasPrefix(line, ":tmi.twitch.tv RECONNECT"):
				return nil
			}
		}
	}
}

// sendLocked writes one IRC line; c.mu must be held.
func (c *Client) sendLocked(line string) {
	if err := c.conn.WriteMessage(websocket.TextMessage, []byte(line+"\r\n")); err != nil {
		c.conn.Close()
	}
}

func rooms(logins []string) string {
	out := make([]string, len(logins))
	for i, l := range logins {
		out[i] = "#" + l
	}
	return strings.Join(out, ",")
}

func (c *Client) logger() *slog.Logger {
	if c.Log != nil {
		return c.Log
	}
	return slog.Default()
}
