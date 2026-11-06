//go:build !windows

package network

import (
	"context"
	"fmt"
	"time"
)

type unsupportedPinger struct{}

func newPinger() Pinger { return unsupportedPinger{} }

func (unsupportedPinger) Ping(context.Context, string) (time.Duration, error) {
	return 0, fmt.Errorf("icmp diagnostics are supported on Windows only")
}
