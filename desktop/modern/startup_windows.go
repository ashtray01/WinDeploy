package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const webView2ClientGUID = "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"

var startupLogPath string

func prepareWebViewDataDir(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return "", fmt.Errorf("WebView2 data directory %s: %w", absolute, err)
	}
	probe, err := os.CreateTemp(absolute, ".windeploy-write-probe-*")
	if err != nil {
		return "", fmt.Errorf("WebView2 data directory %s: %w", absolute, err)
	}
	name := probe.Name()
	closeErr := probe.Close()
	removeErr := os.Remove(name)
	if closeErr != nil {
		return "", closeErr
	}
	if removeErr != nil {
		return "", removeErr
	}
	return absolute, nil
}

// initStartupLog routes framework and internal diagnostics to a bounded local log
// file. GUI builds have no usable stderr, so this is the only durable evidence for
// a failed startup. The file contains no credentials or user documents.
func initStartupLog() (io.Closer, error) {
	log.SetOutput(os.Stderr)
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, fmt.Errorf("startup cache: %w", err)
	}
	return configureStartupLog(filepath.Join(cache, "WinDeploy", "logs"), os.Stderr)
}

// Keep stderr even when opening the durable log fails. Otherwise log.Fatal and
// framework diagnostics disappear exactly when the user needs them most.
func configureStartupLog(dir string, fallback io.Writer) (io.Closer, error) {
	log.SetOutput(fallback)
	startupLogPath = filepath.Join(dir, "startup.log")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("startup log directory: %w", err)
	}
	path := startupLogPath
	if info, statErr := os.Stat(path); statErr == nil && info.Size() > 512*1024 {
		_ = os.Rename(path, path+".1")
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open startup log: %w", err)
	}
	log.SetOutput(io.MultiWriter(fallback, file))
	logStation("запуск приложения")
	return file, nil
}

func logStation(format string, args ...any) {
	log.Printf(format, args...)
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
