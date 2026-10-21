// Package auth implements the Twitch OAuth device code flow, token validation and refresh,
// and stores tokens in the OS keyring.
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const BaseURL = "https://id.twitch.tv/oauth2"

var (
	// ErrInvalidToken means the access token is expired or revoked.
	ErrInvalidToken = errors.New("auth: invalid token")
	// ErrExpired means the device code expired before the user approved it.
	ErrExpired = errors.New("auth: device code expired")
	// ErrDenied means the user rejected the authorization request.
	ErrDenied = errors.New("auth: access denied")
)

type Client struct {
	HTTP      *http.Client
	BaseURL   string
	ClientID  string
	UserAgent string
	Scopes    []string
}

type DeviceCode struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"` // seconds
	Interval        int    `json:"interval"`   // seconds between polls
}

type Token struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `json:"expires_at"` // zero if Twitch did not say
	UserID       string    `json:"user_id"`
	Login        string    `json:"login"`
}

// Validation is the result of /oauth2/validate.
type Validation struct {
	ClientID  string   `json:"client_id"`
	Login     string   `json:"login"`
	UserID    string   `json:"user_id"`
	Scopes    []string `json:"scopes"`
	ExpiresIn int      `json:"expires_in"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type errorResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
}

// StartDevice asks Twitch for a device code; the user enters UserCode at VerificationURI.
func (c *Client) StartDevice(ctx context.Context) (*DeviceCode, error) {
	form := url.Values{"client_id": {c.ClientID}, "scopes": {strings.Join(c.Scopes, " ")}}
	var dc DeviceCode
	if err := c.postForm(ctx, "/device", form, &dc); err != nil {
		return nil, err
	}
	return &dc, nil
}

// PollToken waits until the user approves dc, it expires, or ctx is cancelled.
func (c *Client) PollToken(ctx context.Context, dc *DeviceCode) (*Token, error) {
	interval := time.Duration(dc.Interval) * time.Second
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	form := url.Values{
		"client_id":   {c.ClientID},
		"device_code": {dc.DeviceCode},
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
	}
	for {
		var tr tokenResponse
		err := c.postForm(ctx, "/token", form, &tr)
		if err == nil {
			return newToken(tr), nil
		}
		var apiErr *apiError
		if !errors.As(err, &apiErr) {
			return nil, err
		}
		switch apiErr.Message {
		case "authorization_pending":
		case "slow_down":
			interval += 5 * time.Second
		case "expired_token", "invalid device code":
			return nil, ErrExpired
		case "access_denied":
			return nil, ErrDenied
		default:
			return nil, err
		}
		if dc.ExpiresIn > 0 && time.Now().After(deadline) {
			return nil, ErrExpired
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(interval):
		}
	}
}

// Refresh exchanges a refresh token for a new token pair (public clients need no secret).
func (c *Client) Refresh(ctx context.Context, refreshToken string) (*Token, error) {
	form := url.Values{
		"client_id":     {c.ClientID},
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
	}
	var tr tokenResponse
	if err := c.postForm(ctx, "/token", form, &tr); err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusBadRequest {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	return newToken(tr), nil
}

// Validate checks an access token and returns who it belongs to.
func (c *Client) Validate(ctx context.Context, accessToken string) (*Validation, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base()+"/validate", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "OAuth "+accessToken)
	var v Validation
	if err := c.send(req, &v); err != nil {
		var apiErr *apiError
		if errors.As(err, &apiErr) && apiErr.Status == http.StatusUnauthorized {
			return nil, ErrInvalidToken
		}
		return nil, err
	}
	return &v, nil
}

func newToken(tr tokenResponse) *Token {
	t := &Token{AccessToken: tr.AccessToken, RefreshToken: tr.RefreshToken}
	if tr.ExpiresIn > 0 {
		t.ExpiresAt = time.Now().Add(time.Duration(tr.ExpiresIn) * time.Second)
	}
	return t
}

type apiError struct {
	Status  int
	Message string
}

func (e *apiError) Error() string { return fmt.Sprintf("auth: http %d: %s", e.Status, e.Message) }

func (c *Client) base() string {
	if c.BaseURL != "" {
		return c.BaseURL
	}
	return BaseURL
}

func (c *Client) postForm(ctx context.Context, path string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base()+path, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return c.send(req, out)
}

func (c *Client) send(req *http.Request, out any) error {
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	req.Header.Set("Client-Id", c.ClientID)
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		e := &apiError{Status: resp.StatusCode}
		var er errorResponse
		if json.Unmarshal(raw, &er) == nil && er.Message != "" {
			e.Message = er.Message
		} else {
			e.Message = strings.TrimSpace(string(raw))
		}
		return e
	}
	return json.Unmarshal(raw, out)
}
