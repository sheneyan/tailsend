package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	app := NewApp(nil)
	err := wails.Run(&options.App{
		Title:     "Tailsend",
		Width:     920,
		Height:    640,
		MinWidth:  720,
		MinHeight: 480,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 17, G: 19, B: 24, A: 255},
		OnStartup:        app.startup,
		Bind:             []interface{}{app},
		DragAndDrop: &options.DragAndDrop{
			EnableFileDrop: true,
			// Prevent the WebView from navigating to a dropped file (Linux
			// WebKit would otherwise replace the UI with the file contents).
			// Linux re-installs a GTK uri-list dest in scheduleLinuxFileDrop.
			DisableWebViewDrop: true,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "Tailsend",
				Message: "Send files over Tailscale Taildrop.",
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
