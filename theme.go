package main

import (
	"github.com/LmaoUpal/twitch_drops_rust/internal/config"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// applyTheme matches the native title bar to the UI theme; the page itself switches in CSS.
func (a *App) applyTheme(theme string) {
	if a.ctx == nil {
		return
	}
	switch theme {
	case config.ThemeLight:
		runtime.WindowSetLightTheme(a.ctx)
	case config.ThemeSystem:
		runtime.WindowSetSystemDefaultTheme(a.ctx)
	default:
		runtime.WindowSetDarkTheme(a.ctx)
	}
}
