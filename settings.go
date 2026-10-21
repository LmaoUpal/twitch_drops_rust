package main

import (
	"errors"
	"fmt"
	"log/slog"
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
	if cfg.Twitch.Hashes == nil {
		cfg.Twitch.Hashes = map[string]string{}
	}

	a.mu.Lock()
	cfg.DeviceID = a.cfg.DeviceID
	cfg.AutoMode = a.cfg.AutoMode // owned by SetAutoMode; the UI copy may be stale
	gamesChanged := !slices.Equal(cfg.Games, a.cfg.Games)
	a.cfg = cfg
	path := a.cfgPath
	a.mu.Unlock()

	if path != "" {
		if err := config.Save(path, cfg); err != nil {
			return err
		}
	}
	if gamesChanged {
		slog.Info("Game priority updated: "+strings.Join(cfg.Games, ", "), "tag", "Settings")
		a.RefreshCampaigns()
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
