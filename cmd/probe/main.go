// Command probe runs one Twitch GQL operation with the token saved by the app and
// prints the raw response. Use it to check persisted-query hashes after Twitch rotates them:
//
//	go run ./cmd/probe -op ViewerDropsDashboard -vars '{"fetchRewardCampaigns":false}'
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/auth"
	"github.com/LmaoUpal/twitch_drops_rust/internal/twitch/gql"
)

func main() {
	query := flag.String("query", "", "raw GraphQL query instead of a persisted -op")
	op := flag.String("op", "ViewerDropsDashboard", "GQL operation name (must have a hash in config)")
	vars := flag.String("vars", "{}", "operation variables as JSON")
	flag.Parse()

	if err := run(*op, *query, *vars); err != nil {
		fmt.Fprintln(os.Stderr, "probe:", err)
		os.Exit(1)
	}
}

func run(op, query, rawVars string) error {
	path, err := config.Path()
	if err != nil {
		return err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	tok, err := auth.KeyringStore{Service: "twitch-drops"}.Load("main")
	if err != nil {
		return fmt.Errorf("load token (log in via the app first): %w", err)
	}
	var vars any
	if err := json.Unmarshal([]byte(rawVars), &vars); err != nil {
		return fmt.Errorf("vars: %w", err)
	}

	deviceID := cfg.DeviceID
	if deviceID == "" {
		if deviceID, err = gql.FetchDeviceID(context.Background(), nil, cfg.Twitch.ClientURL, cfg.Twitch.UserAgent); err != nil {
			return err
		}
	}
	c := &gql.Client{
		ClientID:  cfg.Twitch.ClientID,
		UserAgent: cfg.Twitch.UserAgent,
		DeviceID:  deviceID,
		SessionID: gql.NewSessionID(),
		ClientURL: cfg.Twitch.ClientURL,
		Token:     tok.AccessToken,
		Hashes:    cfg.Twitch.Hashes,
	}
	var out json.RawMessage
	do := func() error { return c.Do(context.Background(), op, vars, &out) }
	if query != "" {
		do = func() error { return c.DoQuery(context.Background(), query, vars, &out) }
	}
	if err := do(); err != nil {
		return err
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
