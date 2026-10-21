package gql

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
)

// fixtureClient answers each operation with testdata/<operationName>.json.
func fixtureClient(t *testing.T) *Client {
	t.Helper()
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile("testdata/" + req.OperationName + ".json")
		if err != nil {
			t.Fatalf("no fixture for %s", req.OperationName)
		}
		w.Write(data)
	})
	c.Hashes = map[string]string{"Inventory": "h", "VideoPlayerStreamInfoOverlayChannel": "h", "DirectoryPage_Game": "h"}
	return c
}

func TestInventory(t *testing.T) {
	inv, err := fixtureClient(t).Inventory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := inv.Campaigns
	if len(inv.Claimed) != 1 || inv.Claimed[0].Name != "Rust Hoodie" || inv.Claimed[0].Game.Name != "Rust" || inv.Claimed[0].ClaimedAt.IsZero() {
		t.Errorf("claimed: %+v", inv.Claimed)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 campaigns (game-less one dropped), got %d", len(got))
	}
	general, rivals := got[0], got[1]
	if general.Game.Name != "Rust" || general.Restricted() || !general.AccountLinked || len(general.Drops) != 2 {
		t.Errorf("general: %+v", general)
	}
	if d := general.Drops[1]; d.Name != "Bed" || d.CurrentMinutes != 75 || d.RequiredMinutes != 120 || d.Claimed {
		t.Errorf("bed: %+v", d)
	}
	if !general.Drops[0].Claimed {
		t.Error("rug should be claimed")
	}
	if !rivals.Restricted() || rivals.AllowedChannels[0].Login != "twitchrivals" || rivals.AccountLinked {
		t.Errorf("rivals: %+v", rivals)
	}
	if rivals.Drops[0].InstanceID != "inst-1" {
		t.Errorf("backpack instance id: %+v", rivals.Drops[0])
	}
}

func TestStreamInfo(t *testing.T) {
	ch, err := fixtureClient(t).StreamInfo(context.Background(), "filliolina")
	if err != nil {
		t.Fatal(err)
	}
	if ch.ID != "218217900" || ch.Login != "filliolina" || !ch.Online || ch.Viewers != 53 || ch.GameID != "1264310518" || ch.GameName != "Marvel Rivals" || ch.AvatarURL == "" || ch.Title == "" {
		t.Fatalf("got %+v", ch)
	}
}

func TestGameStreams(t *testing.T) {
	got, err := fixtureClient(t).GameStreams(context.Background(), "rust", 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[1].Login != "bigrust" || got[1].GameID != "263490" || !got[1].DropsEnabled || got[1].Viewers != 900 {
		t.Fatalf("got %+v", got)
	}
}

func TestClaimDrop(t *testing.T) {
	c := fixtureClient(t)
	c.Hashes["DropsPage_ClaimDropRewards"] = "h"
	status, err := c.ClaimDrop(context.Background(), "inst-1")
	if err != nil || status != "ELIGIBLE_FOR_ALL" {
		t.Fatalf("got %q, %v", status, err)
	}
}
