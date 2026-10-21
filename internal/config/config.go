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
	"slices"

	"github.com/pelletier/go-toml/v2"
)

const appDir = "twitch-drops"

type Config struct {
	Games            []string `toml:"games" json:"games"`                        // priority order, first is most wanted
	ExcludedChannels []string `toml:"excluded_channels" json:"excludedChannels"` // logins never to watch
	// PreferredChannels maps a game name to logins watched first, in order (the streamer queue).
	PreferredChannels map[string][]string `toml:"preferred_channels,omitempty" json:"preferredChannels"`
	AutoMode          bool                `toml:"auto_mode" json:"autoMode"` // the miner picks channels itself
	AutoClaim         bool                `toml:"auto_claim" json:"autoClaim"`
	StartWithWindows  bool                `toml:"start_with_windows" json:"startWithWindows"`
	MinimizeToTray    bool                `toml:"minimize_to_tray" json:"minimizeToTray"`
	Notify            Notify              `toml:"notify" json:"notify"`
	Language          string              `toml:"language" json:"language"` // "en" or "ru"
	// RefreshSeconds is how often campaigns and inventory are re-synced.
	RefreshSeconds int `toml:"refresh_seconds" json:"refreshSeconds"`
	// DeviceID is sent as X-Device-Id; generated once on first launch.
	DeviceID string `toml:"device_id,omitempty" json:"-"`
	Twitch   Twitch `toml:"twitch" json:"twitch"`
}

// Notify selects which desktop notifications are shown.
type Notify struct {
	DropClaimed      bool `toml:"drop_claimed" json:"dropClaimed"`
	CampaignStarted  bool `toml:"campaign_started" json:"campaignStarted"`
	AccountNotLinked bool `toml:"account_not_linked" json:"accountNotLinked"`
}

// MinRefreshSeconds is the lowest allowed RefreshSeconds.
const MinRefreshSeconds = 30

type Twitch struct {
	ClientID  string `toml:"client_id,omitempty" json:"clientId"`
	UserAgent string `toml:"user_agent,omitempty" json:"userAgent"`
	// ClientURL is the web origin of the client app, sent as Origin/Referer.
	ClientURL string `toml:"client_url,omitempty" json:"clientUrl"`
	// Scopes requested during device login.
	Scopes []string `toml:"scopes,omitempty" json:"scopes"`
	// Hashes maps GQL operation name to its persisted-query sha256 hash.
	Hashes map[string]string `toml:"hashes,omitempty" json:"hashes"`
}

// Default returns the settings used on first launch.
// The client ID is the Twitch Android TV app (the Android phone and web IDs are rejected
// by the device flow, checked 2026-10-21). GQL hashes follow DevilXD/TwitchDropsMiner constants.py.
// Update them here or in config.toml when Twitch rotates them.
func Default() Config {
	return Config{
		Games:          []string{"Rust"},
		AutoMode:       true,
		AutoClaim:      true,
		MinimizeToTray: true,
		Notify:         Notify{DropClaimed: true, AccountNotLinked: true},
		Language:       "en",
		RefreshSeconds: 300,
		Twitch: Twitch{
			ClientID:  "ue6666qo983tsx6so1t0vnawi233wa",
			ClientURL: "https://android.tv.twitch.tv",
			UserAgent: "Mozilla/5.0 (Linux; Android 7.1; Smart Box C1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/153.0.0.0 Safari/537.36",
			Scopes:    []string{"channel_read", "chat:read", "user_blocks_edit", "user_blocks_read", "user_follows_edit", "user_read"},
			Hashes: map[string]string{
				"ViewerDropsDashboard":                 "c16bb890cc8ce7647a96ee69cd313d423a378a3dedadf630a1017cde18975feb",
				"DropCampaignDetails":                  "039277bf98f3130929262cc7c6efd9c141ca3749cb6dca442fc8ead9a53f77c1",
				"Inventory":                            "8337eb8541b314040b0edde0c09c5c7a2783ba1960aa9edfbf3bac16d0fec404",
				"DropCurrentSessionContext":            "4d06b702d25d652afb9ef835d2a550031f1cf762b193523a92166f40ea3d142b",
				"DropsPage_ClaimDropRewards":           "a455deea71bdc9015b78eb49f4acfbce8baa7ccbedd28e549bb025bd0f751930",
				"DirectoryPage_Game":                   "86bcceb4e8b1a51256ff8eed8bd8aae4acacf80d737efe904f84f3aeadf8cafd",
				"PlaybackAccessToken":                  "ed230aa1e33e07eebb8928504583da78a5173989fadfb1ac94be06a04f3cdbe9",
				"VideoPlayerStreamInfoOverlayChannel":  "198492e0857f6aedead9665c81c5a06d67b25b58034649687124083ff288597d",
				"DropsHighlightService_AvailableDrops": "782dad0f032942260171d2d80a654f88bdd0c5a9dddc392e9bc92218a0f42d20",
				"DirectoryGameRedirect":                "1f0300090caceec51f33c5e20647aceff9017f740f223c3c532ba6fa59f6b6cc",
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

// Save writes cfg to path. Twitch constants equal to the built-in defaults are left out,
// so updated defaults in a new app version take effect; only user overrides persist.
func Save(path string, cfg Config) error {
	cfg.Twitch = overrides(cfg.Twitch, Default().Twitch)
	data, err := toml.Marshal(cfg)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func overrides(t, def Twitch) Twitch {
	out := Twitch{Hashes: map[string]string{}}
	if t.ClientID != def.ClientID {
		out.ClientID = t.ClientID
	}
	if t.UserAgent != def.UserAgent {
		out.UserAgent = t.UserAgent
	}
	if t.ClientURL != def.ClientURL {
		out.ClientURL = t.ClientURL
	}
	if !slices.Equal(t.Scopes, def.Scopes) {
		out.Scopes = t.Scopes
	}
	for op, h := range t.Hashes {
		if def.Hashes[op] != h {
			out.Hashes[op] = h
		}
	}
	if len(out.Hashes) == 0 {
		out.Hashes = nil
	}
	return out
}
