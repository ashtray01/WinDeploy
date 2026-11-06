//go:build windows

package packages

import (
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// WindowsWinget probes the Windows Package Manager. The probe is cached because
// starting the App Installer alias is relatively slow. Installs themselves run
// through the injected Runner so exit codes and timeouts are handled uniformly.
type WindowsWinget struct {
	mu        sync.Mutex
	checked   bool
	available bool
	version   string
}

// Available implements WingetProbe.
func (w *WindowsWinget) Available(ctx context.Context) (bool, string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.checked {
		return w.available, w.version
	}
	w.checked = true
	path, err := exec.LookPath("winget.exe")
	if err != nil {
		return false, ""
	}
	probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, path, "--version")
	cmd.SysProcAttr = hiddenProcessAttributes()
	output, err := cmd.Output()
	if err != nil {
		return false, ""
	}
	version := strings.TrimSpace(string(output))
	if version == "" || len(version) > 64 {
		return false, ""
	}
	w.available, w.version = true, version
	return true, version
}
