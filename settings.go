package main

import (
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/LmaoUpal/twitch_drops_rust/internal/activity"
	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
)

const eventActivity = "activity:entry"

// GetSettings returns the current settings.
func (a *App) GetSettings() config.Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

// SaveSettings replaces the settings and persists them. Twitch client changes apply to
// new requests; the device id is kept.
func (a *App) SaveSettings(cfg config.Config) error {
	if cfg.RefreshSeconds < config.MinRefreshSeconds {
		return fmt.Errorf("refresh interval must be at least %d seconds", config.MinRefreshSeconds)
	}
	cfg.Games = cleanList(cfg.Games, false)
	cfg.ExcludedChannels = cleanList(cfg.ExcludedChannels, true)
	cfg.PreferredChannels = cleanQueues(cfg.PreferredChannels, cfg.Games)
	if cfg.Twitch.Hashes == nil {
		cfg.Twitch.Hashes = map[string]string{}
	}
	if _, ok := messages[cfg.Language]; !ok {
		cfg.Language = "en"
	}

	a.mu.Lock()
	autostartChanged := cfg.StartWithWindows != a.cfg.StartWithWindows
	a.mu.Unlock()
	if autostartChanged {
		if err := setAutostart(cfg.StartWithWindows); err != nil {
			return fmt.Errorf("start with Windows: %w", err)
		}
	}

	a.mu.Lock()
	cfg.DeviceID = a.cfg.DeviceID
	cfg.AutoMode = a.cfg.AutoMode               // owned by SetAutoMode; the UI copy may be stale
	cfg.Points.Channels = a.cfg.Points.Channels // owned by the points channel API, same reason
	pointsChanged := cfg.Points.Enabled != a.cfg.Points.Enabled || cfg.Points.JoinChat != a.cfg.Points.JoinChat
	gamesChanged := !slices.Equal(cfg.Games, a.cfg.Games)
	langChanged := cfg.Language != a.cfg.Language
	queuesChanged := !maps.EqualFunc(cfg.PreferredChannels, a.cfg.PreferredChannels, slices.Equal)
	a.cfg = cfg
	path := a.cfgPath
	a.mu.Unlock()

	if path != "" {
		if err := config.Save(path, cfg); err != nil {
			return err
		}
	}
	if langChanged {
		a.updateTray()
	}
	if gamesChanged {
		slog.Info("Game priority updated: "+strings.Join(cfg.Games, ", "), "tag", "Settings")
		a.RefreshCampaigns()
	}
	if pointsChanged {
		a.setPoints(func(*PointsState) {})
		a.kickPoints()
	}
	if queuesChanged && !gamesChanged {
		slog.Info("Streamer queue updated", "tag", "Settings")
		a.kickAuto()
	}
	return nil
}

// DefaultSettings returns the built-in defaults, for "Reset to defaults" in Advanced.
func (a *App) DefaultSettings() config.Config {
	return config.Default()
}

// OpenConfigFolder shows the folder holding config.toml in Explorer.
func (a *App) OpenConfigFolder() error {
	a.mu.Lock()
	path := a.cfgPath
	a.mu.Unlock()
	if path == "" {
		return errors.New("config location unknown")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return exec.Command("explorer", dir).Start()
}

// GetActivity returns the activity feed, oldest first.
func (a *App) GetActivity() []activity.Entry {
	return a.feed.Entries()
}

// ExportActivity asks for a file name and writes the activity feed there as text.
// It returns the chosen path, or "" when the dialog was cancelled.
func (a *App) ExportActivity() (string, error) {
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		DefaultFilename: "drops-miner-activity.txt",
		Filters:         []runtime.FileFilter{{DisplayName: "Text files", Pattern: "*.txt"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	return path, os.WriteFile(path, []byte(ActivityText(a.feed.Entries())), 0o644)
}

// ActivityText formats entries one per line, as in the Activity log screen.
func ActivityText(entries []activity.Entry) string {
	var b strings.Builder
	for _, e := range entries {
		fmt.Fprintf(&b, "%s  %-5s  %s", e.Time.Format("2006-01-02 15:04:05"), strings.ToUpper(e.Level), e.Message)
		if e.Tag != "" {
			fmt.Fprintf(&b, "  [%s]", e.Tag)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// cleanList trims entries and drops blanks and duplicates (case-insensitive).
// cleanQueues keeps streamer queues of wanted games only, keyed by the game's name as spelled
// in games, with clean logins; empty queues are dropped.
func cleanQueues(in map[string][]string, games []string) map[string][]string {
	out := map[string][]string{}
	for g, logins := range in {
		for _, want := range games {
			if strings.EqualFold(g, want) {
				if l := cleanList(append(out[want], logins...), true); len(l) > 0 {
					out[want] = l
				}
			}
		}
	}
	return out
}

func cleanList(in []string, lower bool) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if lower {
			s = strings.ToLower(strings.TrimPrefix(s, "@"))
		}
		k := strings.ToLower(s)
		if s == "" || seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, s)
	}
	return out
}
