// Package config loads and saves user settings and Twitch API constants as TOML.
//
// Client ID and GQL persisted-query hashes are not public API and change from time
// to time, so they live here (editable by the user) rather than in code.
package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

const appDir = "twitch-drops"

type Config struct {
	Games            []string `toml:"games" json:"games"`                        // priority order, first is most wanted
	ExcludedChannels []string `toml:"excluded_channels" json:"excludedChannels"` // logins never to watch
	AutoClaim        bool     `toml:"auto_claim" json:"autoClaim"`
	Twitch           Twitch   `toml:"twitch" json:"twitch"`
}

type Twitch struct {
	ClientID  string `toml:"client_id" json:"clientId"`
	UserAgent string `toml:"user_agent" json:"userAgent"`
	// Scopes requested during device login.
	Scopes []string `toml:"scopes" json:"scopes"`
	// Hashes maps GQL operation name to its persisted-query sha256 hash.
	Hashes map[string]string `toml:"hashes" json:"hashes"`
}

// Default returns the settings used on first launch.
// The client ID is the Twitch Android TV app (the Android phone and web IDs are rejected
// by the device flow, checked 2026-10-21). GQL hashes are unverified until phase 2.
// Update them here or in config.toml when Twitch rotates them.
func Default() Config {
	return Config{
		Games:     []string{"Rust"},
		AutoClaim: true,
		Twitch: Twitch{
			ClientID:  "ue6666qo983tsx6so1t0vnawi233wa",
			UserAgent: "Mozilla/5.0 (Linux; Android 7.1; Smart Box C1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
			Scopes:    []string{"channel_read", "chat:read", "user_blocks_edit", "user_blocks_read", "user_follows_edit", "user_read"},
			Hashes: map[string]string{
				"ViewerDropsDashboard":                "5a4da2ab3d5b47c9f9ce864e727b2cb346af1e3ea8b897fe8f704a97ff017619",
				"DropCampaignDetails":                 "039277bf98f3130929262cc7c6efd9c141ca3749cb6dca442fc8ead9a53f77c1",
				"Inventory":                           "09acb7d3d7e605a92bdfdcc465f6aa481b71c234d8686a9ba38ea5ed51507592",
				"DropCurrentSessionContext":           "4d06b702d25d652afb9ef835d2a550031f1cf762b193523a92166f40ea3d142b",
				"DropsPage_ClaimDropRewards":          "a455deea71bdc9015b78eb49f4acfbce8baa7ccbedd28e549bb025bd0f751930",
				"DirectoryPage_Game":                  "c7c9d5aad09155c4161d2382092dc44610367f3536aac39019ec2582ae5065f9",
				"PlaybackAccessToken":                 "3093517e37e4f4cb48906155bcd894150aef92617939236d2508f3375ab732ce",
				"VideoPlayerStreamInfoOverlayChannel": "a5f2e34d626a9f4f5c0204f910bab2194948a9502089be558bb6e779a9e1b3d2",
			},
		},
	}
}

// Path returns the config file location, e.g. %APPDATA%\twitch-drops\config.toml.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, appDir, "config.toml"), nil
}

// Load reads path, falling back to Default for a missing file or missing fields.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	defaults := cfg.Twitch.Hashes
	cfg.Twitch.Hashes = nil
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}
	// Keep defaults for operations the file does not mention.
	if cfg.Twitch.Hashes == nil {
		cfg.Twitch.Hashes = map[string]string{}
	}
	for op, h := range defaults {
		if _, ok := cfg.Twitch.Hashes[op]; !ok {
			cfg.Twitch.Hashes[op] = h
		}
	}
	return cfg, nil
}

func Save(path string, cfg Config) error {
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
