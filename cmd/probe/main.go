// Command probe checks Twitch behaviour with the token saved by the app.
//
//	go run ./cmd/probe -op ViewerDropsDashboard -vars '{"fetchRewardCampaigns":false}'
//	go run ./cmd/probe -query 'query { currentUser { login } }'
//	go run ./cmd/probe -facepunch
//	go run ./cmd/probe -watch somechannel -minutes 5
//	go run ./cmd/probe -pubsub raid.123,predictions-channel-v1.123 -minutes 5
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
	"github.com/LmaoUpal/twitch_drops_rust/internal/facepunch"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/auth"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/gql"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/watcher"
)

func main() {
	fp := flag.Bool("facepunch", false, "fetch and print the parsed twitch.facepunch.com schedule instead")
	watch := flag.String("watch", "", "watch this channel login and print drop progress every minute")
	minutes := flag.Int("minutes", 5, "with -watch: how long to watch")
	query := flag.String("query", "", "raw GraphQL query instead of a persisted -op")
	op := flag.String("op", "ViewerDropsDashboard", "GQL operation name (must have a hash in config)")
	vars := flag.String("vars", "{}", "operation variables as JSON")
	pubsub := flag.String("pubsub", "", "comma-separated PubSub topics to listen to; prints messages for -minutes")
	flag.Parse()

	runner := func() error { return run(*op, *query, *vars) }
	switch {
	case *fp:
		runner = runFacepunch
	case *watch != "":
		runner = func() error { return runWatch(*watch, time.Duration(*minutes)*time.Minute) }
	case *pubsub != "":
		runner = func() error { return runPubSub(strings.Split(*pubsub, ","), time.Duration(*minutes)*time.Minute) }
	}
	if err := runner(); err != nil {
		fmt.Fprintln(os.Stderr, "probe:", err)
		os.Exit(1)
	}
}

func runFacepunch() error {
	campaigns, err := facepunch.Fetch(context.Background(), nil)
	if err != nil {
		return err
	}
	return printJSON(campaigns)
}

func run(op, query, rawVars string) error {
	c, _, err := client()
	if err != nil {
		return err
	}
	var vars any
	if err := json.Unmarshal([]byte(rawVars), &vars); err != nil {
		return fmt.Errorf("vars: %w", err)
	}
	var out json.RawMessage
	do := func() error { return c.Do(context.Background(), op, vars, &out) }
	if query != "" {
		do = func() error { return c.DoQuery(context.Background(), query, vars, &out) }
	}
	if err := do(); err != nil {
		return err
	}
	return printJSON(out)
}

// runWatch watches login for d and prints the current drop session each minute,
// to confirm that segment polling really advances drop progress.
func runWatch(login string, d time.Duration) error {
	c, tok, err := client()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	ch, err := c.StreamInfo(ctx, login)
	if err != nil {
		return err
	}
	if !ch.Online {
		return fmt.Errorf("%s is offline", login)
	}
	fmt.Printf("watching %s (id %s, game %s, broadcast %s) for %s\n", ch.Login, ch.ID, ch.GameID, ch.BroadcastID, d)

	w := &watcher.Watcher{
		GQL: c, UserID: tok.UserID, UserAgent: c.UserAgent,
		Report: func(s watcher.Status) {
			fmt.Printf("%s segments=%d spade=%d %s\n", s.LastPoll.Format("15:04:05"), s.Segments, s.SpadeEvents, s.Err)
		},
	}
	done := make(chan error, 1)
	go func() { done <- w.Watch(ctx, watcher.Stream{Channel: ch}) }()

	tick := time.NewTicker(time.Minute)
	defer tick.Stop()
	for {
		select {
		case err := <-done:
			if errors.Is(err, context.DeadlineExceeded) {
				err = nil
			}
			s, _ := c.CurrentDrop(context.Background(), ch.ID)
			fmt.Printf("final drop session: %+v\n", s)
			return err
		case <-tick.C:
			s, err := c.CurrentDrop(ctx, ch.ID)
			fmt.Printf("drop session: %+v err=%v\n", s, err)
		}
	}
}

func client() (*gql.Client, *auth.Token, error) {
	path, err := config.Path()
	if err != nil {
		return nil, nil, err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, nil, err
	}
	tok, err := auth.KeyringStore{Service: "twitch-drops"}.Load("main")
	if err != nil {
		return nil, nil, fmt.Errorf("load token (log in via the app first): %w", err)
	}
	deviceID := cfg.DeviceID
	if deviceID == "" {
		if deviceID, err = gql.FetchDeviceID(context.Background(), nil, cfg.Twitch.ClientURL, cfg.Twitch.UserAgent); err != nil {
			return nil, nil, err
		}
	}
	return &gql.Client{
		ClientID:  cfg.Twitch.ClientID,
		UserAgent: cfg.Twitch.UserAgent,
		DeviceID:  deviceID,
		SessionID: gql.NewSessionID(),
		ClientURL: cfg.Twitch.ClientURL,
		Token:     tok.AccessToken,
		Hashes:    cfg.Twitch.Hashes,
		Retries:   2,
	}, tok, nil
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
