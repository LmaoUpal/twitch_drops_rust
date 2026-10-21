package gql

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Client{
		Endpoint: srv.URL,
		ClientID: "cid",
		Token:    "tok",
		Hashes:   map[string]string{"Inventory": "abc123"},
		Retries:  1,
	}
}

func TestDoSendsPersistedQueryAndDecodes(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Client-Id") != "cid" || r.Header.Get("Authorization") != "OAuth tok" {
			t.Errorf("bad headers: %v", r.Header)
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.OperationName != "Inventory" || req.Extensions.PersistedQuery.Sha256Hash != "abc123" {
			t.Errorf("bad request: %+v", req)
		}
		w.Write([]byte(`{"data":{"currentUser":{"id":"42"}}}`))
	})

	var out struct {
		CurrentUser struct{ ID string } `json:"currentUser"`
	}
	if err := c.Do(context.Background(), "Inventory", map[string]any{"fetchRewardCampaigns": false}, &out); err != nil {
		t.Fatal(err)
	}
	if out.CurrentUser.ID != "42" {
		t.Fatalf("got %+v", out)
	}
}

func TestDoUnknownOperation(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) { t.Error("must not send") })
	if err := c.Do(context.Background(), "Nope", nil, nil); err == nil {
		t.Fatal("want error")
	}
}

func TestDoRetriesServerErrors(t *testing.T) {
	calls := 0
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.Write([]byte(`{"data":{}}`))
	})
	if err := c.Do(context.Background(), "Inventory", nil, nil); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatalf("want 2 calls, got %d", calls)
	}
}

func TestDoUnauthorized(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errors":[{"message":"Unauthorized"}]}`))
	})
	if err := c.Do(context.Background(), "Inventory", nil, nil); !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("want ErrUnauthorized, got %v", err)
	}
}
