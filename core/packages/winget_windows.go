//go:build windows

package packages

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

var wingetVersionPattern = regexp.MustCompile(`^v?[0-9][0-9A-Za-z.+-]{0,40}$`)

// WindowsWinget probes the Windows Package Manager. The probe is cached because
// starting the App Installer is relatively slow. Windows App Execution Aliases
// cannot always be opened with os.Stat (Go's exec.LookPath fails), so cmd.exe and
// PowerShell fallbacks are used to resolve the alias.
type WindowsWinget struct {
	mu        sync.Mutex
	checked   bool
	available bool
	version   string
	path      string
}

// Available implements WingetProbe.
func (w *WindowsWinget) Available(ctx context.Context) (bool, string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.checked {
		return w.available, w.version
	}
	w.checked = true
	probeCtx, cancel := context.WithTimeout(ctx, 40*time.Second)
	defer cancel()
	for _, candidate := range wingetCandidates() {
		if version, ok := runWingetVersion(probeCtx, candidate); ok {
			w.available, w.version, w.path = true, version, candidate
			return true, version
		}
	}
	if version, ok := runWingetVersion(probeCtx, "cmd.exe", "/d", "/s", "/c", "winget", "--version"); ok {
		w.available, w.version, w.path = true, version, "winget"
		return true, version
	}
	if version, ok := runWingetVersion(probeCtx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "winget --version"); ok {
		w.available, w.version, w.path = true, version, "winget"
		return true, version
	}
	return false, ""
}

// Path returns the resolved winget.exe path or the bare "winget" alias when only
// cmd.exe can resolve it. Callers must invoke Available first.
func (w *WindowsWinget) Path() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.path
}

func runWingetVersion(ctx context.Context, executable string, args ...string) (string, bool) {
	if ctx.Err() != nil {
		return "", false
	}
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.SysProcAttr = hiddenProcessAttributes()
	output, err := cmd.Output()
	if err != nil {
		return "", false
	}
	for _, line := range strings.Split(string(output), "\n") {
		value := strings.TrimSpace(strings.TrimPrefix(line, "\ufeff"))
		if wingetVersionPattern.MatchString(value) {
			return value, true
		}
	}
	return "", false
}

// wingetCandidates lists plausible winget.exe locations in priority order.
func wingetCandidates() []string {
	seen := map[string]bool{}
	result := make([]string, 0, 4)
	add := func(path string) {
		if path == "" {
			return
		}
		key := strings.ToLower(filepath.Clean(path))
		if seen[key] {
			return
		}
		seen[key] = true
		if info, err := os.Stat(path); err == nil && info.Mode().IsRegular() {
			result = append(result, filepath.Clean(path))
		}
	}
	if path, err := exec.LookPath("winget.exe"); err == nil {
		add(path)
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		add(filepath.Join(local, "Microsoft", "WindowsApps", "winget.exe"))
	}
	programFiles := []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)")}
	patterns := []string{
		"Microsoft.DesktopAppInstaller_*_x64__8wekyb3d8bbwe\\winget.exe",
		"Microsoft.DesktopAppInstaller_*_x86__8wekyb3d8bbwe\\winget.exe",
		"Microsoft.DesktopAppInstaller_*_neutral_~_8wekyb3d8bbwe\\winget.exe",
		"Microsoft.DesktopAppInstaller_*_arm64__8wekyb3d8bbwe\\winget.exe",
	}
	var packageMatches []string
	for _, root := range programFiles {
		if root == "" {
			continue
		}
		for _, pattern := range patterns {
			matches, err := filepath.Glob(filepath.Join(root, "WindowsApps", pattern))
			if err == nil {
				packageMatches = append(packageMatches, matches...)
			}
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(packageMatches)))
	for _, match := range packageMatches {
		add(match)
	}
	return result
}
