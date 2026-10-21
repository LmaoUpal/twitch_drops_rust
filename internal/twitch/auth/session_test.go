package auth

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

type memStore map[string]*Token

func (m memStore) Load(a string) (*Token, error) {
	if t, ok := m[a]; ok {
		cp := *t
		return &cp, nil
	}
	return nil, ErrNoToken
}
func (m memStore) Save(a string, t *Token) error { cp := *t; m[a] = &cp; return nil }
func (m memStore) Delete(a string) error         { delete(m, a); return nil }

// fakeTwitch accepts access token "good"/"new" and refresh token "ref".
func fakeTwitch(t *testing.T) *Client {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /validate", func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "OAuth good", "OAuth new":
			w.Write([]byte(`{"login":"vadim","user_id":"42"}`))
		default:
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"status":401,"message":"invalid access token"}`))
		}
	})
	mux.HandleFunc("POST /token", func(w http.ResponseWriter, r *http.Request) {
		if r.FormValue("refresh_token") == "ref" {
			w.Write([]byte(`{"access_token":"new","refresh_token":"ref2"}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"status":400,"message":"Invalid refresh token"}`))
	})
	return newTestClient(t, mux)
}

func TestRestore(t *testing.T) {
	tests := []struct {
		name       string
		saved      *Token
		wantAccess string
		wantErr    error
		wantSaved  bool
	}{
		{"nothing saved", nil, "", ErrNoToken, false},
		{"valid token", &Token{AccessToken: "good"}, "good", nil, true},
		{"expired token refreshed", &Token{AccessToken: "old", RefreshToken: "ref"}, "new", nil, true},
		{"expired token, refresh rejected", &Token{AccessToken: "old", RefreshToken: "stale"}, "", ErrNoToken, false},
		{"expired token, no refresh", &Token{AccessToken: "old"}, "", ErrNoToken, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := memStore{}
			if tt.saved != nil {
				store["main"] = tt.saved
			}
			s := &Session{Client: fakeTwitch(t), Store: store, Account: "main"}
			tok, err := s.Restore(context.Background())
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("want err %v, got %v", tt.wantErr, err)
			}
			if tt.wantErr == nil && (tok.AccessToken != tt.wantAccess || tok.Login != "vadim" || tok.UserID != "42") {
				t.Fatalf("got %+v", tok)
			}
			saved, ok := store["main"]
			if ok != tt.wantSaved {
				t.Fatalf("saved=%v, want %v", ok, tt.wantSaved)
			}
			if ok && saved.AccessToken != tt.wantAccess {
				t.Fatalf("stored %q, want %q", saved.AccessToken, tt.wantAccess)
			}
		})
	}
}

func TestCompleteAndLogout(t *testing.T) {
	store := memStore{}
	s := &Session{Client: fakeTwitch(t), Store: store, Account: "main"}
	tok, err := s.Complete(context.Background(), &Token{AccessToken: "good"})
	if err != nil || tok.Login != "vadim" {
		t.Fatalf("got %+v, %v", tok, err)
	}
	if store["main"] == nil || store["main"].UserID != "42" {
		t.Fatal("token not saved with identity")
	}
	if err := s.Logout(); err != nil || len(store) != 0 {
		t.Fatalf("logout: %v, store %v", err, store)
	}
}
