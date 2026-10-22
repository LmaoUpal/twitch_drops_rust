// Package gql is a minimal client for Twitch's private GraphQL endpoint using persisted queries.
package gql

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const Endpoint = "https://gql.twitch.tv/gql"

type Client struct {
	HTTP      *http.Client
	Endpoint  string
	ClientID  string
	UserAgent string
	DeviceID  string
	SessionID string            // random per app run, sent as Client-Session-Id
	ClientURL string            // sent as Origin and Referer, e.g. https://android.tv.twitch.tv
	Token     string            // OAuth access token, without the "OAuth " prefix
	Hashes    map[string]string // operation name -> persisted query sha256
	Retries   int               // extra attempts on network errors and 5xx
}

type request struct {
	OperationName string      `json:"operationName,omitempty"`
	Query         string      `json:"query,omitempty"`
	Variables     any         `json:"variables,omitempty"`
	Extensions    *extensions `json:"extensions,omitempty"`
}

type extensions struct {
	PersistedQuery persistedQuery `json:"persistedQuery"`
}

type persistedQuery struct {
	Version    int    `json:"version"`
	Sha256Hash string `json:"sha256Hash"`
}

type response struct {
	Data   json.RawMessage `json:"data"`
	Errors []Error         `json:"errors"`
}

// Error is a GraphQL-level error returned with HTTP 200.
type Error struct {
	Message string `json:"message"`
}

func (e Error) Error() string { return "gql: " + e.Message }

// ErrUnauthorized means the token is missing, expired or revoked.
var ErrUnauthorized = errors.New("gql: unauthorized")

// Do runs operation op with vars and decodes its "data" object into out.
func (c *Client) Do(ctx context.Context, op string, vars, out any) error {
	hash, ok := c.Hashes[op]
	if !ok {
		return fmt.Errorf("gql: no persisted query hash for %s", op)
	}
	body, err := json.Marshal(request{
		OperationName: op,
		Variables:     vars,
		Extensions:    &extensions{PersistedQuery: persistedQuery{Version: 1, Sha256Hash: hash}},
	})
	if err != nil {
		return err
	}
	return c.send(ctx, body, out)
}

// DoQuery runs a raw GraphQL query string. Twitch only accepts these from some clients (the TV
// client does); it is for debugging (cmd/probe) and operations with no known persisted hash.
func (c *Client) DoQuery(ctx context.Context, query string, vars, out any) error {
	body, err := json.Marshal(request{Query: query, Variables: vars})
	if err != nil {
		return err
	}
	return c.send(ctx, body, out)
}

func (c *Client) send(ctx context.Context, body []byte, out any) error {

	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(attempt) * time.Second):
			}
		}
		var retry bool
		retry, lastErr = c.do(ctx, body, out)
		if lastErr == nil || !retry {
			return lastErr
		}
	}
	return lastErr
}

func (c *Client) do(ctx context.Context, body []byte, out any) (retry bool, err error) {
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = Endpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Client-Id", c.ClientID)
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	if c.DeviceID != "" {
		req.Header.Set("X-Device-Id", c.DeviceID)
	}
	if c.SessionID != "" {
		req.Header.Set("Client-Session-Id", c.SessionID)
	}
	if c.ClientURL != "" {
		req.Header.Set("Origin", c.ClientURL)
		req.Header.Set("Referer", c.ClientURL)
	}
	req.Header.Set("Accept-Language", "en-US")
	if c.Token != "" {
		req.Header.Set("Authorization", "OAuth "+c.Token)
	}

	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return ctx.Err() == nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return true, err
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return false, ErrUnauthorized
	case resp.StatusCode >= 500:
		return true, fmt.Errorf("gql: http %d", resp.StatusCode)
	case resp.StatusCode != http.StatusOK:
		return false, fmt.Errorf("gql: http %d: %s", resp.StatusCode, raw)
	}

	var r response
	if err := json.Unmarshal(raw, &r); err != nil {
		return false, fmt.Errorf("gql: decode: %w", err)
	}
	if len(r.Errors) > 0 {
		if strings.Contains(strings.ToLower(r.Errors[0].Message), "unauthorized") {
			return false, ErrUnauthorized
		}
		// "service timeout"/"service error" are transient on Twitch's side.
		return strings.HasPrefix(r.Errors[0].Message, "service"), r.Errors[0]
	}
	if out == nil {
		return false, nil
	}
	if err := json.Unmarshal(r.Data, out); err != nil {
		return false, fmt.Errorf("gql: decode data: %w", err)
	}
	return false, nil
}
