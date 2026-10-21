package main

import (
	"context"
	_ "embed"
	"fmt"
	"log/slog"
	"sync"

	"fyne.io/systray"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed build/windows/icon.ico
var trayIcon []byte

// tray holds the system tray menu items; nil fields mean the tray is not ready yet.
type tray struct {
	mu     sync.Mutex
	status *systray.MenuItem
	pause  *systray.MenuItem
}

// startTray shows the tray icon. systray runs its own message loop, so it gets a goroutine.
func (a *App) startTray() {
	go systray.Run(a.onTrayReady, nil)
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("Twitch Drops Miner")
	status := systray.AddMenuItem("Starting…", "")
	status.Disable()
	systray.AddSeparator()
	pause := systray.AddMenuItem("Pause mining", "")
	open := systray.AddMenuItem("Open", "")
	systray.AddSeparator()
	quit := systray.AddMenuItem("Quit", "")
	systray.SetOnTapped(a.showWindow) // left click opens, right click shows the menu

	a.tray.mu.Lock()
	a.tray.status, a.tray.pause = status, pause
	a.tray.mu.Unlock()
	a.updateTray()

	go func() {
		for {
			select {
			case <-pause.ClickedCh:
				a.SetPaused(!a.GetAutoState().Paused)
			case <-open.ClickedCh:
				a.showWindow()
			case <-quit.ClickedCh:
				a.quit()
				return
			}
		}
	}()
}

// updateTray mirrors the miner state into the tray tooltip and menu.
func (a *App) updateTray() {
	a.tray.mu.Lock()
	defer a.tray.mu.Unlock()
	if a.tray.status == nil {
		return
	}
	w, auto := a.GetWatchState(), a.GetAutoState()
	text := "Idle — nothing to watch"
	switch {
	case auto.Mode == modeAuto && auto.Paused:
		text = "Paused"
	case w.State == watchActive || w.State == watchStarting:
		name := w.Channel.DisplayName
		if name == "" {
			name = w.Channel.Login
		}
		text = "Watching " + name
		if s := w.Session; s.RequiredMinutes > 0 {
			text += fmt.Sprintf(" — %d / %d min", s.CurrentMinutes, s.RequiredMinutes)
		}
	case w.State == watchOffline:
		text = "Channel offline"
	}
	a.tray.status.SetTitle(text)
	systray.SetTooltip("Twitch Drops Miner — " + text)

	if auto.Mode != modeAuto {
		a.tray.pause.SetTitle("Manual mode")
		a.tray.pause.Disable()
		return
	}
	a.tray.pause.Enable()
	if auto.Paused {
		a.tray.pause.SetTitle("Resume mining")
	} else {
		a.tray.pause.SetTitle("Pause mining")
	}
}

func (a *App) showWindow() {
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

func (a *App) quit() {
	a.mu.Lock()
	a.quitting = true
	a.mu.Unlock()
	runtime.Quit(a.ctx)
}

// beforeClose hides the window instead of closing when "Minimize to tray on close" is on.
func (a *App) beforeClose(ctx context.Context) bool {
	a.mu.Lock()
	hide := a.cfg.MinimizeToTray && !a.quitting
	first := !a.hiddenOnce
	a.hiddenOnce = a.hiddenOnce || hide
	a.mu.Unlock()
	if !hide {
		return false
	}
	runtime.WindowHide(ctx)
	if first {
		slog.Info("Window hidden — mining continues in the tray", "tag", "App")
		a.notify("Still running in the tray", "Twitch Drops Miner keeps mining. Right-click the tray icon to quit.")
	}
	return true
}

func (a *App) shutdown(context.Context) {
	systray.Quit()
}
