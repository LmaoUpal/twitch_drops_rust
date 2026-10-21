package main

import (
	"context"
	_ "embed"
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
	open   *systray.MenuItem
	quit   *systray.MenuItem
}

// startTray shows the tray icon. systray runs its own message loop, so it gets a goroutine.
func (a *App) startTray() {
	go systray.Run(a.onTrayReady, nil)
}

func (a *App) onTrayReady() {
	systray.SetIcon(trayIcon)
	systray.SetTooltip("Twitch Drops Miner")
	status := systray.AddMenuItem(a.tr("tray.starting"), "")
	status.Disable()
	systray.AddSeparator()
	pause := systray.AddMenuItem(a.tr("tray.pause"), "")
	open := systray.AddMenuItem(a.tr("tray.open"), "")
	systray.AddSeparator()
	quit := systray.AddMenuItem(a.tr("tray.quit"), "")
	systray.SetOnTapped(a.showWindow) // left click opens, right click shows the menu

	a.tray.mu.Lock()
	a.tray.status, a.tray.pause, a.tray.open, a.tray.quit = status, pause, open, quit
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
	a.tray.open.SetTitle(a.tr("tray.open"))
	a.tray.quit.SetTitle(a.tr("tray.quit"))
	text := a.tr("tray.idle")
	switch {
	case auto.Mode == modeAuto && auto.Paused:
		text = a.tr("tray.paused")
	case w.State == watchActive || w.State == watchStarting:
		name := w.Channel.DisplayName
		if name == "" {
			name = w.Channel.Login
		}
		text = a.tr("tray.watching", name)
		if s := w.Session; s.RequiredMinutes > 0 {
			text += a.tr("tray.progress", s.CurrentMinutes, s.RequiredMinutes)
		}
	case w.State == watchOffline:
		text = a.tr("tray.offline")
	}
	a.tray.status.SetTitle(text)
	systray.SetTooltip("Twitch Drops Miner — " + text)

	if auto.Mode != modeAuto {
		a.tray.pause.SetTitle(a.tr("tray.manual"))
		a.tray.pause.Disable()
		return
	}
	a.tray.pause.Enable()
	if auto.Paused {
		a.tray.pause.SetTitle(a.tr("tray.resume"))
	} else {
		a.tray.pause.SetTitle(a.tr("tray.pause"))
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
		a.notify(a.tr("notify.trayTitle"), a.tr("notify.trayBody"))
	}
	return true
}

func (a *App) shutdown(context.Context) {
	systray.Quit()
}
