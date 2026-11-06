package main

import (
	"embed"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	smokePort := flag.Int("smoke-port", 0, "opt-in localhost WebView2 debugging for UI smoke tests")
	flag.Parse()
	if *smokePort != 0 && (*smokePort < 1024 || *smokePort > 65535) {
		log.Fatal("invalid smoke port")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		log.Fatal("user cache directory unavailable")
	}
	options := application.WindowsOptions{WebviewUserDataPath: filepath.Join(cache, "WinDeploy", "WebView2")}
	if *smokePort != 0 {
		options.AdditionalBrowserArgs = []string{"--remote-debugging-address=127.0.0.1", "--remote-debugging-port=" + strconv.Itoa(*smokePort)}
	}
	app := application.New(application.Options{
		Name: "WinDeploy", Description: "Подготовка рабочих станций Windows",
		Windows: options,
		Assets:  application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "WinDeploy", Width: 1366, Height: 840, MinWidth: 1024, MinHeight: 640,
		Windows:          application.WindowsWindow{Theme: application.Dark},
		BackgroundColour: application.NewRGB(14, 18, 26), URL: "/", DevToolsEnabled: *smokePort != 0,
	})
	if err := app.Run(); err != nil {
		log.Fatal(err)
	}
}
