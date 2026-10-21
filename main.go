package main

import (
	"embed"
	"log/slog"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"

	"github.com/LmaoUpal/twitch_drops_rust/internal/activity"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	// Every slog record at Info or above also lands in the in-app Activity log.
	feed := activity.NewFeed(1000)
	slog.SetDefault(slog.New(&activity.Handler{Next: slog.NewTextHandler(os.Stderr, nil), Feed: feed}))

	app := NewApp(feed)

	err := wails.Run(&options.App{
		Title:     "Twitch Drops Miner",
		Width:     1100,
		Height:    720,
		MinWidth:  860,
		MinHeight: 560,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 18, A: 1},
		OnStartup:        app.startup,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}
