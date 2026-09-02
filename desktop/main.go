package main

import (
	"embed"
	"log"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
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
			// Linux/macOS: stop the WebView from opening the dropped file.
			// Windows (Wails v2.15): must stay false or AllowExternalDrag(false)
			// swallows Explorer drops and OnFileDrop never runs. JS preventDefault
			// still blocks WebView2 from navigating.
			DisableWebViewDrop: webViewDropDisabled(),
		},
		Linux: &linux.Options{
			Icon:             appIcon,
			ProgramName:      "Tailsend",
			WebviewGpuPolicy: linux.WebviewGpuPolicyNever,
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarHiddenInset(),
			About: &mac.AboutInfo{
				Title:   "Tailsend",
				Message: "Send files over Tailscale Taildrop.",
				Icon:    appIcon,
			},
		},
	})
	if err != nil {
		log.Fatal(err)
	}
}
