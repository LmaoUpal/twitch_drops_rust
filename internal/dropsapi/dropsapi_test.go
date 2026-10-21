package dropsapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestFetchAndParse(t *testing.T) {
	data, err := os.ReadFile("testdata/v2_drops.json")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(data) }))
	defer srv.Close()

	cat, err := Fetch(context.Background(), nil, srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if cat.UpdatedAt.IsZero() || len(cat.Campaigns) != 2 {
		t.Fatalf("got %d campaigns, updated %v", len(cat.Campaigns), cat.UpdatedAt)
	}
	restricted, open := cat.Campaigns[0], cat.Campaigns[1]
	if restricted.Game.Name != "Rise Online" || restricted.Game.Slug != "rise-online" || restricted.Game.BoxArtURL == "" || restricted.Owner == "" {
		t.Errorf("game/owner: %+v", restricted)
	}
	if len(restricted.Channels) != 2 || restricted.Channels[0].Login == "" {
		t.Errorf("channels: %+v", restricted.Channels)
	}
	if len(restricted.Drops) != 1 || restricted.Drops[0].RequiredMinutes != 240 || restricted.Drops[0].ImageURL == "" || restricted.Drops[0].Name != "Regalia of True Faith" {
		t.Errorf("drops: %+v", restricted.Drops)
	}
	if len(open.Channels) != 0 {
		t.Errorf("allow disabled must mean any channel, got %+v", open.Channels)
	}
}

func TestFetchHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadGateway) }))
	defer srv.Close()
	if _, err := Fetch(context.Background(), nil, srv.URL); err == nil {
		t.Fatal("want error")
	}
}
