package gql

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func pointsClient(t *testing.T) *Client {
	c := fixtureClient(t)
	c.Hashes = map[string]string{"ChannelPointsContext": "h", "ClaimCommunityPoints": "h", "ChannelFollows": "h"}
	return c
}

func TestChannelPoints(t *testing.T) {
	info, err := pointsClient(t).ChannelPoints(context.Background(), "twitchrivals")
	if err != nil {
		t.Fatal(err)
	}
	if info.ChannelID != "197886470" || info.Balance != 1250 || info.ClaimID != "claim-1" || !info.Enabled {
		t.Fatalf("got %+v", info)
	}
}

func TestClaimPoints(t *testing.T) {
	bal, err := pointsClient(t).ClaimPoints(context.Background(), "197886470", "claim-1")
	if err != nil || bal != 1300 {
		t.Fatalf("got %d, %v", bal, err)
	}
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"claimCommunityPoints":{"claim":null,"currentPoints":null,"error":{"code":"NOT_FOUND"}}}}`))
	})
	c.Hashes = map[string]string{"ClaimCommunityPoints": "h"}
	if _, err := c.ClaimPoints(context.Background(), "1", "x"); err == nil || !strings.Contains(err.Error(), "NOT_FOUND") {
		t.Fatalf("want NOT_FOUND error, got %v", err)
	}
}

func TestFollows(t *testing.T) {
	got, err := pointsClient(t).Follows(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Login != "ohnepixel" || got[0].ID != "43683025" || got[1].DisplayName != "TwitchRivals" {
		t.Fatalf("got %+v", got)
	}
}

func TestFollowsPages(t *testing.T) {
	var cursors []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				Cursor string `json:"cursor"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		cursors = append(cursors, req.Variables.Cursor)
		next := req.Variables.Cursor == ""
		w.Write([]byte(`{"data":{"user":{"follows":{"edges":[{"cursor":"p` + req.Variables.Cursor + `","node":{"id":"1","login":"a"}}],"pageInfo":{"hasNextPage":` + map[bool]string{true: "true", false: "false"}[next] + `}}}}}`))
	})
	c.Hashes = map[string]string{"ChannelFollows": "h"}
	got, err := c.Follows(context.Background())
	if err != nil || len(got) != 2 || len(cursors) != 2 || cursors[1] != "p" {
		t.Fatalf("got %d channels, cursors %q, err %v", len(got), cursors, err)
	}
}

func TestFollowAndIsFollowing(t *testing.T) {
	var queries []string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req request
		json.NewDecoder(r.Body).Decode(&req)
		queries = append(queries, req.Query)
		switch {
		case strings.Contains(req.Query, "followUser"):
			w.Write([]byte(`{"data":{"followUser":{"follow":{"followedAt":"2026-10-22T00:00:00Z"},"error":null}}}`))
		default:
			w.Write([]byte(`{"data":{"user":{"id":"1","self":{"follower":null}}}}`))
		}
	})
	following, err := c.IsFollowing(context.Background(), "someone")
	if err != nil || following {
		t.Fatalf("IsFollowing: %v %v", following, err)
	}
	if err := c.Follow(context.Background(), "1"); err != nil {
		t.Fatal(err)
	}
	if len(queries) != 2 || queries[0] != isFollowingQuery || queries[1] != followQuery {
		t.Fatalf("raw queries sent: %q", queries)
	}
}

func TestMakePrediction(t *testing.T) {
	var input map[string]any
	reply := `{"data":{"makePrediction":{"error":null}}}`
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				Input map[string]any `json:"input"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		input = req.Variables.Input
		w.Write([]byte(reply))
	})
	c.Hashes = map[string]string{"MakePrediction": "h"}
	if err := c.MakePrediction(context.Background(), "ev", "o1", 100); err != nil {
		t.Fatal(err)
	}
	if input["eventID"] != "ev" || input["outcomeID"] != "o1" || input["points"] != float64(100) || len(input["transactionID"].(string)) != 32 {
		t.Fatalf("input %+v", input)
	}
	reply = `{"data":{"makePrediction":{"error":{"code":"MUST_ACCEPT_TOS"}}}}`
	if err := c.MakePrediction(context.Background(), "ev", "o1", 100); err != ErrMustAcceptTOS {
		t.Fatalf("want ErrMustAcceptTOS, got %v", err)
	}
}

func TestActivePredictions(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"channel":{"id":"42","activePredictionEvents":[{"id":"ev","status":"ACTIVE","title":"Win?","createdAt":"2026-10-22T08:21:32Z","predictionWindowSeconds":1200,"outcomes":[{"id":"o1","title":"Yes","totalPoints":500,"totalUsers":7},{"id":"o2","title":"No","totalPoints":100,"totalUsers":2}]}]}}}`))
	})
	got, err := c.ActivePredictions(context.Background(), "x")
	if err != nil || len(got) != 1 {
		t.Fatalf("%+v %v", got, err)
	}
	p := got[0]
	if p.ID != "ev" || p.ChannelID != "42" || p.Window.Minutes() != 20 || len(p.Outcomes) != 2 || p.Outcomes[0].Users != 7 || p.Outcomes[1].Points != 100 {
		t.Fatalf("%+v", p)
	}
}

func TestJoinRaid(t *testing.T) {
	var input map[string]any
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Variables struct {
				Input map[string]any `json:"input"`
			} `json:"variables"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		input = req.Variables.Input
		w.Write([]byte(`{"data":{"joinRaid":{"raidID":"r1"}}}`))
	})
	c.Hashes = map[string]string{"JoinRaid": "h"}
	if err := c.JoinRaid(context.Background(), "r1"); err != nil || input["raidID"] != "r1" {
		t.Fatalf("%v %+v", err, input)
	}
}
