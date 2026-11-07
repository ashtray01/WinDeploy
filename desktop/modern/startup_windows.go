package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"windeploy/core/trust"
)

const (
	webView2ClientGUID = "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"
	fallbackMarkerName = ".windeploy-fallback-accepted"
)

var startupLogPath string

// prepareWebViewDataDir validates an explicitly selected directory. It rejects
// reparse points but leaves ACL policy to the operator for explicit paths.
func prepareWebViewDataDir(dir string) (string, error) {
	absolute, err := ensureWebViewDataDir(dir)
	if err != nil {
		return "", err
	}
	return probeWebViewDataDir(absolute)
}

// prepareTrustedWebViewDataDir is used for automatic WebView2 cache selection.
// The directory must be owned by the current user, must not be writable by other
// principals and must not traverse a link/junction. This prevents a fallback from
// silently sharing WebView2 cookies/cache with another local account.
func prepareTrustedWebViewDataDir(dir string) (string, error) {
	absolute, err := ensureWebViewDataDir(dir)
	if err != nil {
		return "", err
	}
	if err := trust.Check(context.Background(), absolute, true); err != nil {
		return "", fmt.Errorf("WebView2 data directory is not trusted: %w", err)
	}
	return probeWebViewDataDir(absolute)
}

func ensureWebViewDataDir(dir string) (string, error) {
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return "", fmt.Errorf("WebView2 data directory %s: %w", absolute, err)
	}
	if err := rejectReparseAncestors(absolute); err != nil {
		return "", fmt.Errorf("WebView2 data directory %s: %w", absolute, err)
	}
	return absolute, nil
}

func rejectReparseAncestors(path string) error {
	current := filepath.Clean(path)
	for {
		ptr, err := windows.UTF16PtrFromString(current)
		if err != nil {
			return fmt.Errorf("path cannot be inspected: %w", err)
		}
		attributes, err := windows.GetFileAttributes(ptr)
		if err != nil {
			return fmt.Errorf("path cannot be inspected: %w", err)
		}
		if attributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
			return fmt.Errorf("path contains a symbolic link or junction")
		}
		parent := filepath.Dir(current)
		if parent == current {
			return nil
		}
		current = parent
	}
}

func probeWebViewDataDir(absolute string) (string, error) {
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
// writable and trusted. A stale ACL, sandbox principal or antivirus lock must not
// prevent the application from starting: a fresh trusted directory under the
// user cache, the user temp root, and finally the portable executable directory
// are tried. Program Files is never used as a portable fallback. No existing
// directory is deleted or repaired.
func prepareDefaultWebViewDataDir(cache, temp, executableDir string) (string, bool, error) {
	candidates := []string{
		filepath.Join(cache, "WinDeploy", "WebView2"),
		filepath.Join(cache, "WinDeploy", "WebView2-fallback"),
	}
	if temp != "" {
		candidates = append(candidates, filepath.Join(temp, "WinDeploy", "WebView2"))
	}
	if executableDir != "" && !isProgramFilesPath(executableDir) {
		candidates = append(candidates, filepath.Join(executableDir, "WinDeploy-WebView2Data"))
	}
	var firstErr, lastErr error
	for index, candidate := range candidates {
		dir, err := prepareTrustedWebViewDataDir(candidate)
		if err == nil {
			if index > 0 {
				logStation("каталог WebView2 по умолчанию недоступен; используется резервный каталог: %s", dir)
				return dir, true, nil
			}
			return dir, false, nil
		}
		logStation("кандидат каталога WebView2 отклонён (%s): %v", candidate, err)
		if firstErr == nil {
			firstErr = err
		}
		lastErr = err
	}
	return "", false, fmt.Errorf("каталог WebView2 недоступен (primary: %v): %w", firstErr, lastErr)
}

func isProgramFilesPath(dir string) bool {
	cleaned := strings.ToLower(filepath.Clean(dir))
	for _, name := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432"} {
		root := strings.TrimSpace(os.Getenv(name))
		if root == "" {
			continue
		}
		root = strings.ToLower(filepath.Clean(root))
		if cleaned == root || strings.HasPrefix(cleaned, root+string(os.PathSeparator)) {
			return true
		}
	}
	return false
}

func fallbackAccepted(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, fallbackMarkerName))
	return err == nil
}

func markFallbackAccepted(dir string) error {
	return os.WriteFile(filepath.Join(dir, fallbackMarkerName), []byte("accepted\n"), 0o600)
}

// confirmFallback asks once per trusted fallback directory. The marker contains
// no secrets and lets subsequent launches reuse the same cache without another
// prompt, preserving the selected WebView2 profile.
func confirmFallback(dir string) bool {
	if fallbackAccepted(dir) {
		return true
	}
	if !showFallbackChoice(dir) {
		return false
	}
	if err := markFallbackAccepted(dir); err != nil {
		logStation("не удалось сохранить выбор резервного каталога: %v", err)
	}
	return true
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

func handleWebViewDataDirFailure(err error) {
	logStation("каталог WebView2 недоступен: %v", err)
	showStartupErrorMessage("WinDeploy — каталог WebView2 недоступен", "Не удалось подготовить безопасный каталог данных WebView2.\n\n"+err.Error()+"\n\nЗакройте другое окно WinDeploy и проверьте права на %LOCALAPPDATA%\\WinDeploy. При необходимости укажите доверенный локальный каталог флагом --webview-data-dir.\n\nСуществующие данные не удалялись.\n\nЖурнал запуска: "+startupLogPath)
}
