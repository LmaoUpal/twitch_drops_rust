package main

import (
	"context"
	"log/slog"

	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
)

// App is the bridge between the Go backend and the React frontend; its exported
// methods are callable from the UI via generated wailsjs bindings.
type App struct {
	ctx     context.Context
	cfgPath string
	cfg     config.Config
}

func NewApp() *App {
	return &App{cfg: config.Default()}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	path, err := config.Path()
	if err != nil {
		slog.Error("config path", "err", err)
		return
	}
	a.cfgPath = path
	cfg, err := config.Load(path)
	if err != nil {
		slog.Error("load config, using defaults", "path", path, "err", err)
	}
	a.cfg = cfg
}

// GetConfig returns the current settings.
func (a *App) GetConfig() config.Config {
	return a.cfg
}

// SetGames replaces the game priority list and persists it.
func (a *App) SetGames(games []string) error {
	a.cfg.Games = games
	return config.Save(a.cfgPath, a.cfg)
}
