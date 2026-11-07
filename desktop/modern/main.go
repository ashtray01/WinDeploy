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
	if *smokePort != 0 && (*smokePort < 1024 || *smokePort > 65535) {
		logStation("некорректный порт отладки")
		showStartupError()
		os.Exit(2)
	}
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
		dataDir = filepath.Join(cache, "WinDeploy", "WebView2")
	}
	dataDir, err = prepareWebViewDataDir(dataDir)
	if err != nil {
		logStation("каталог WebView2 недоступен: %v", err)
		showStartupErrorMessage("WinDeploy — каталог WebView2 недоступен", "Не удалось записать данные WebView2.\n\n"+err.Error()+"\n\nУкажите доступный каталог флагом --webview-data-dir. Существующие данные не удалялись.")
		os.Exit(4)
	}
	logStation("каталог WebView2: %s", dataDir)
	options := application.WindowsOptions{WebviewUserDataPath: dataDir}
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
		BackgroundColour: application.NewRGB(14, 18, 26), URL: "/", DevToolsEnabled: *smokePort != 0,
	})
	logStation("окно создано, запуск Wails")
	if err := app.Run(); err != nil {
		logStation("Wails завершился с ошибкой: %v", err)
		showStartupError()
		log.Fatal(err)
	}
	logStation("приложение завершено")
}
