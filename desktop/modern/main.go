package main

import (
	"embed"
	"errors"
	"flag"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/wailsapp/wails/v3/pkg/application"
	"windeploy/core/inventory"
	"windeploy/modern/service"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	smokePort := flag.Int("smoke-port", 0, "opt-in localhost WebView2 debugging for UI smoke tests")
	profileDir := flag.String("profiles", "", "read YAML profiles from this directory instead of embedded examples")
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
	backend := service.New(inventory.Native{})
	if *profileDir != "" {
		backend = service.NewWithProfiles(inventory.Native{}, os.DirFS(*profileDir))
	}
	app := application.New(application.Options{
		Name: "WinDeploy", Description: "Подготовка рабочих станций Windows",
		Windows:  options,
		Assets:   application.AssetOptions{Handler: application.AssetFileServerFS(assets)},
		Services: []application.Service{application.NewService(backend)},
		ErrorHandler: func(err error) {
			service.RuntimeErrorHandler(backend)(err)
			var fatal *application.FatalError
			if errors.As(err, &fatal) {
				showStartupError()
			}
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "WinDeploy", Width: 1366, Height: 840, MinWidth: 1024, MinHeight: 640,
		Windows:          application.WindowsWindow{Theme: application.Dark},
		BackgroundColour: application.NewRGB(14, 18, 26), URL: "/", DevToolsEnabled: *smokePort != 0,
	})
	if err := app.Run(); err != nil {
		showStartupError()
		log.Fatal(err)
	}
}
