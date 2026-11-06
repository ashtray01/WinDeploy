package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const webView2ClientGUID = "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"

var startupLogPath string

// initStartupLog routes framework and internal diagnostics to a bounded local log
// file. GUI builds have no usable stderr, so this is the only durable evidence for
// a failed startup. The file contains no credentials or user documents.
func initStartupLog() {
	cache, err := os.UserCacheDir()
	if err != nil {
		log.SetOutput(io.Discard)
		return
	}
	dir := filepath.Join(cache, "WinDeploy", "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.SetOutput(io.Discard)
		return
	}
	path := filepath.Join(dir, "startup.log")
	if info, statErr := os.Stat(path); statErr == nil && info.Size() > 512*1024 {
		_ = os.Rename(path, path+".1")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.SetOutput(io.Discard)
		return
	}
	startupLogPath = path
	log.SetOutput(file)
	logStation("запуск приложения")
}

func logStation(format string, args ...any) {
	line := fmt.Sprintf(format, args...)
	if startupLogPath != "" {
		if file, err := os.OpenFile(startupLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
			_, _ = fmt.Fprintf(file, "%s %s\n", time.Now().Format(time.RFC3339), line)
			_ = file.Close()
		}
	}
}

// webView2RuntimeVersion reads the Evergreen WebView2 runtime version from the
// machine or per-user registration. It does not install or repair anything.
func webView2RuntimeVersion() (string, bool) {
	candidates := []struct {
		root registry.Key
		path string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\` + webView2ClientGUID},
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\EdgeUpdate\Clients\` + webView2ClientGUID},
		{registry.CURRENT_USER, `Software\Microsoft\EdgeUpdate\Clients\` + webView2ClientGUID},
	}
	for _, candidate := range candidates {
		key, err := registry.OpenKey(candidate.root, candidate.path, registry.QUERY_VALUE|registry.WOW64_64KEY)
		if err != nil {
			continue
		}
		version, _, err := key.GetStringValue("pv")
		key.Close()
		if err == nil && version != "" {
			return version, true
		}
	}
	return "", false
}

// showWebView2Missing explains the only external prerequisite the application
// needs. It deliberately does not download or install the runtime.
func showWebView2Missing(version string) {
	text := "Для работы WinDeploy требуется Microsoft Edge WebView2 Runtime.\n\n" +
		"Установите Evergreen WebView2 Runtime с сайта Microsoft, затем запустите приложение снова.\n\n" +
		"Обнаруженный WebView2 Runtime: " + version + "\n" +
		"Журнал запуска: " + startupLogPath + "\n\n" +
		"WinDeploy не изменял систему и не устанавливал компоненты автоматически."
	showStartupErrorMessage("WinDeploy — отсутствует WebView2 Runtime", text)
}

func showStartupError() {
	text := "Не удалось запустить WinDeploy.\n\n" +
		"Проверьте Microsoft Edge WebView2 Runtime, права на пользовательский кэш %LOCALAPPDATA%\\WinDeploy и отсутствие другого экземпляра приложения.\n\n" +
		"Журнал запуска: " + startupLogPath + "\n\n" +
		"Системные изменения не выполнялись."
	showStartupErrorMessage("WinDeploy — ошибка запуска", text)
}

func showStartupErrorMessage(title, text string) {
	titlePtr, _ := windows.UTF16PtrFromString(title)
	textPtr, _ := windows.UTF16PtrFromString(text)
	_, _ = windows.MessageBox(0, textPtr, titlePtr, windows.MB_OK|windows.MB_ICONERROR)
}
