package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"windeploy/core/packages"
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

// prepareDefaultWebViewDataDir keeps an existing user profile when it is
// writable. A stale ACL, sandbox principal or antivirus lock must not prevent the
// application from starting: a fresh directory under the same user cache and
// finally under the user temp root are safe fallbacks. No existing directory is
// deleted or repaired.
func prepareDefaultWebViewDataDir(cache, temp string) (string, error) {
	candidates := []string{
		filepath.Join(cache, "WinDeploy", "WebView2"),
		filepath.Join(cache, "WinDeploy", "WebView2-fallback"),
	}
	if temp != "" {
		candidates = append(candidates, filepath.Join(temp, "WinDeploy", "WebView2"))
	}
	var firstErr, lastErr error
	for index, candidate := range candidates {
		dir, err := prepareWebViewDataDir(candidate)
		if err == nil {
			if index > 0 {
				logStation("каталог WebView2 по умолчанию недоступен; используется резервный каталог: %s", dir)
			}
			return dir, nil
		}
		if firstErr == nil {
			firstErr = err
		}
		lastErr = err
	}
	return "", fmt.Errorf("каталог WebView2 недоступен (primary: %v): %w", firstErr, lastErr)
}

// initStartupLog routes framework and internal diagnostics to a bounded local log
// file. GUI builds have no usable stderr, so this is the only durable evidence for
// a failed startup. The file contains no credentials or user documents.
func initStartupLog(dir string) (io.Closer, error) {
	log.SetOutput(os.Stderr)
	if dir != "" {
		return configureStartupLog(dir, os.Stderr)
	}
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
	// Write the durable file first. A GUI process usually has no valid stderr, and
	// io.MultiWriter stops at the first error; putting stderr first would leave the
	// startup log empty even though the file opened successfully.
	log.SetOutput(io.MultiWriter(file, fallback))
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

// handleWebViewDataDirFailure offers an explicit UAC restart when the process
// cannot write any WebView2 data directory and the failure is a permission error.
// It never elevates without a user click and never repairs ACLs automatically.
func handleWebViewDataDirFailure(err error) {
	logStation("каталог WebView2 недоступен: %v", err)
	if isPermissionFailure(err) && !processIsElevated() {
		const (
			idYes = 6
			text  = "WinDeploy не удалось записать данные WebView2.\n\n" +
				"Возможная причина — недостаточные права на пользовательский каталог.\n\n" +
				"Нажмите «Да», чтобы перезапустить WinDeploy от имени администратора.\n" +
				"Будет показан запрос UAC; текущее окно закроется.\n" +
				"«Нет» закроет приложение без изменений.\n\n" +
				"Существующие данные WebView2 не удаляются. Системные изменения\n" +
				"не выполнялись.\n\n" +
				"Журнал запуска: %s"
		)
		titlePtr, _ := windows.UTF16PtrFromString("WinDeploy — требуется запуск от имени администратора")
		message := fmt.Sprintf(text, startupLogPath)
		textPtr, _ := windows.UTF16PtrFromString(message)
		result, _ := windows.MessageBox(0, textPtr, titlePtr, windows.MB_YESNO|windows.MB_ICONWARNING|windows.MB_DEFBUTTON2)
		if result == idYes {
			if relaunchErr := packages.RelaunchElevated(""); relaunchErr != nil {
				logStation("не удалось перезапустить с правами администратора: %v", relaunchErr)
				showStartupErrorMessage("WinDeploy — не удалось перезапустить", "Запрос прав администратора не выполнен или был отменён.\n\nЗакройте окно и запустите WinDeploy вручную от имени администратора.\n\nЖурнал запуска: "+startupLogPath)
				os.Exit(4)
			}
			os.Exit(0)
		}
		return
	}
	showStartupErrorMessage("WinDeploy — каталог WebView2 недоступен", "Не удалось записать данные WebView2 ни в пользовательский, ни в резервный временный каталог.\n\n"+err.Error()+"\n\nУкажите доступный каталог флагом --webview-data-dir. Существующие данные не удалялись.\n\nЖурнал запуска: "+startupLogPath)
}

func processIsElevated() bool {
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false
	}
	defer token.Close()
	return token.IsElevated()
}

func isPermissionFailure(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, windows.ERROR_ACCESS_DENIED)
}
