package gql

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
)

// FetchDeviceID loads clientURL and returns the "unique_id" cookie Twitch assigns to new
// visitors; Twitch expects it back as X-Device-Id.
func FetchDeviceID(ctx context.Context, httpClient *http.Client, clientURL, userAgent string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, clientURL, nil)
	if err != nil {
		return "", err
	}
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	resp.Body.Close()
	for _, c := range resp.Cookies() {
		if c.Name == "unique_id" && c.Value != "" {
			return c.Value, nil
		}
	}
	return "", errors.New("gql: no unique_id cookie")
}

// NewSessionID returns a random 16-char hex Client-Session-Id.
func NewSessionID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}
