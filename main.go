package main

import (
	"embed"
	"log/slog"
	"os"
	"slices"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/LmaoUpal/twitch_drops_rust/internal/activity"
)

//go:embed all:frontend/dist
var assets embed.FS

// minimizedFlag starts the app hidden in the tray; the Windows autostart entry passes it.
const minimizedFlag = "--minimized"

func main() {
	// Every slog record at Info or above also lands in the in-app Activity log.
	feed := activity.NewFeed(1000)
	slog.SetDefault(slog.New(&activity.Handler{Next: slog.NewTextHandler(os.Stderr, nil), Feed: feed}))

	app := NewApp(feed)

	err := wails.Run(&options.App{
		Title:       "Twitch Drops Miner",
		Width:       1100,
		Height:      720,
		MinWidth:    860,
		MinHeight:   560,
		StartHidden: slices.Contains(os.Args[1:], minimizedFlag),
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:   &options.RGBA{R: 15, G: 15, B: 18, A: 1},
		OnStartup:          app.startup,
		OnBeforeClose:      app.beforeClose,
		OnShutdown:         app.shutdown,
		SingleInstanceLock: singleInstance(app),
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// singleInstance keeps one miner per machine: a second one on the same account would fight
// over the watched channel. Dev builds skip it so they can run next to the installed app.
func singleInstance(app *App) *options.SingleInstanceLock {
	if devBuild {
		return nil
	}
	return &options.SingleInstanceLock{
		UniqueId:               "twitch-drops-miner-3f6c1d2e",
		OnSecondInstanceLaunch: func(options.SecondInstanceData) { app.showWindow() },
	}
}
