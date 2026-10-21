package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, mux *http.ServeMux) *Client {
	t.Helper()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, ClientID: "cid", Scopes: []string{"user_read", "chat:read"}}
}

func TestStartDevice(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /device", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("client_id") != "cid" || r.FormValue("scopes") != "user_read chat:read" {
			t.Errorf("bad form: %v", r.Form)
		}
		w.Write([]byte(`{"device_code":"dev","user_code":"ABCDEFGH","verification_uri":"https://www.twitch.tv/activate","expires_in":1800,"interval":5}`))
	})
	dc, err := newTestClient(t, mux).StartDevice(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if dc.UserCode != "ABCDEFGH" || dc.DeviceCode != "dev" || dc.Interval != 5 {
		t.Fatalf("got %+v", dc)
	}
}

func TestPollTokenWaitsForApproval(t *testing.T) {
	calls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.FormValue("device_code") != "dev" {
			t.Errorf("bad device_code %q", r.FormValue("device_code"))
		}
		if calls < 3 {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status":400,"message":"authorization_pending"}`))
			return
		}
		w.Write([]byte(`{"access_token":"acc","refresh_token":"ref","expires_in":3600}`))
	})
	tok, err := newTestClient(t, mux).PollToken(context.Background(), &DeviceCode{DeviceCode: "dev", ExpiresIn: 60})
	if err != nil {
		t.Fatal(err)
	}
	if tok.AccessToken != "acc" || tok.RefreshToken != "ref" || tok.ExpiresAt.IsZero() || calls != 3 {
		t.Fatalf("got %+v after %d calls", tok, calls)
	}
}

func TestPollTokenErrors(t *testing.T) {
	for msg, want := range map[string]error{
		"expired_token": ErrExpired,
		"access_denied": ErrDenied,
	} {
		mux := http.NewServeMux()
		mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status":400,"message":"` + msg + `"}`))
		})
		_, err := newTestClient(t, mux).PollToken(context.Background(), &DeviceCode{DeviceCode: "dev"})
		if !errors.Is(err, want) {
			t.Errorf("%s: want %v, got %v", msg, want, err)
		}
	}
}

func TestPollTokenCancelled(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"status":400,"message":"authorization_pending"}`))
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := newTestClient(t, mux).PollToken(ctx, &DeviceCode{DeviceCode: "dev", Interval: 5})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want context.Canceled, got %v", err)
	}
}

func TestValidate(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /validate", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "OAuth good" {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"status":401,"message":"invalid access token"}`))
			return
		}
		w.Write([]byte(`{"client_id":"cid","login":"vadim","user_id":"42","scopes":[],"expires_in":0}`))
	})
	c := newTestClient(t, mux)
	v, err := c.Validate(context.Background(), "good")
	if err != nil || v.Login != "vadim" || v.UserID != "42" {
		t.Fatalf("got %+v, %v", v, err)
	}
	if _, err := c.Validate(context.Background(), "bad"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}

func TestRefresh(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("grant_type") != "refresh_token" {
			t.Errorf("bad grant_type %q", r.FormValue("grant_type"))
		}
		if r.FormValue("refresh_token") != "ref" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status":400,"message":"Invalid refresh token"}`))
			return
		}
		w.Write([]byte(`{"access_token":"new","refresh_token":"ref2"}`))
	})
	c := newTestClient(t, mux)
	tok, err := c.Refresh(context.Background(), "ref")
	if err != nil || tok.AccessToken != "new" || tok.RefreshToken != "ref2" {
		t.Fatalf("got %+v, %v", tok, err)
	}
	if _, err := c.Refresh(context.Background(), "stale"); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("want ErrInvalidToken, got %v", err)
	}
}
