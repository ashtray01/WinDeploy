//go:build windows

package network

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
	"time"
)

// pingPinger uses the built-in Windows ping.exe. It does not require raw socket
// privileges and works on Windows 7 SP1. Only the exit code is interpreted, so
// localized output does not matter.
type pingPinger struct{}

func newPinger() Pinger { return pingPinger{} }

func (pingPinger) Ping(ctx context.Context, host string) (time.Duration, error) {
	if host == "" {
		return 0, fmt.Errorf("host is required")
	}
	start := time.Now()
	cmd := exec.CommandContext(ctx, "ping.exe", "-n", "1", "-w", strconv.Itoa(1500), host)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Run(); err != nil {
		if _, isExit := err.(*exec.ExitError); isExit {
			return 0, fmt.Errorf("host did not answer icmp")
		}
		return 0, ErrPingUnavailable
	}
	return time.Since(start), nil
}
