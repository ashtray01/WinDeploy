//go:build windows

package inventory

import (
	"context"
	"testing"

	"golang.org/x/sys/windows"
)

func TestModernBootIdentityStableReadOnly(t *testing.T) {
	if windows.RtlGetVersion().MajorVersion < 10 {
		t.Skip("Modern Windows test")
	}
	first, err := (Native{}).BootID(context.Background())
	if err != nil || len(first) != 32 {
		t.Fatalf("boot identity unavailable: %q %v", first, err)
	}
	second, err := (Native{}).BootID(context.Background())
	if err != nil || first != second {
		t.Fatalf("unstable identity: %q %q %v", first, second, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (Native{}).BootID(ctx); err == nil {
		t.Fatal("cancel ignored")
	}
}
