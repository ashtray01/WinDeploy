package main

import (
	"embed"
	"errors"
	"flag"
	"log"
	"os"

	"github.com/wailsapp/wails/v3/pkg/application"
	"windeploy/core/inventory"
	"windeploy/modern/service"
)

//go:embed all:frontend/dist
var assets embed.FS

func main() {
	profileDir := flag.String("profiles", "", "read YAML profiles from this directory instead of embedded examples")
	webviewDir := flag.String("webview-data-dir", "", "use an explicit writable WebView2 data directory (does not delete existing data)")
	logDir := flag.String("log-dir", "", "write startup.log in this directory instead of the default user cache")
	flag.Parse()
	startupLog, startupErr := initStartupLog(*logDir)
	if startupErr != nil {
		logStation("не удалось открыть журнал запуска: %v", startupErr)
	}
	if startupLog != nil {
		defer startupLog.Close()
	}
	if err := validateSmoke(); err != nil {
		logStation("некорректный порт отладки")
		showStartupError()
		os.Exit(2)
	}
	smoke := smokeEnabled()
	if version, ok := webView2RuntimeVersion(); !ok {
		logStation("WebView2 Runtime не обнаружен")
		showWebView2Missing("не найден")
		os.Exit(3)
	} else {
		logStation("WebView2 Runtime %s", version)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		logStation("каталог пользовательского кэша недоступен")
		showStartupError()
		os.Exit(4)
	}
	dataDir := *webviewDir
	if dataDir == "" {
		dataDir, err = prepareDefaultWebViewDataDir(cache, os.TempDir())
	} else {
		dataDir, err = prepareWebViewDataDir(dataDir)
	}
	if err != nil {
		handleWebViewDataDirFailure(err)
		os.Exit(4)
	}
	logStation("каталог WebView2: %s", dataDir)
	options := application.WindowsOptions{WebviewUserDataPath: dataDir, WndClass: mainWindowClass}
	if smoke {
		options.AdditionalBrowserArgs = smokeBrowserArgs()
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
			logStation("ошибка среды: %v", err)
			var fatal *application.FatalError
			if errors.As(err, &fatal) {
				showStartupError()
			}
		},
	})
	app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title: "WinDeploy", Width: 1366, Height: 840, MinWidth: 1024, MinHeight: 640,
		Windows:          application.WindowsWindow{Theme: application.Dark},
		BackgroundColour: application.NewRGB(14, 18, 26), URL: "/", DevToolsEnabled: smoke,
	})
	logStation("окно создано, запуск Wails")
	applyWindowIcon()
	if err := app.Run(); err != nil {
		logStation("Wails завершился с ошибкой: %v", err)
		showStartupError()
		log.Fatal(err)
	}
	logStation("приложение завершено")
}
