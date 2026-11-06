//go:build windows

package inventory

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"unsafe"
)

func TestNativeSystemReadOnly(t *testing.T) {
	want := uintptr(48)
	if runtime.GOARCH == "386" {
		want = 36
	}
	if unsafe.Sizeof(nativeSystemInfo{}) != want {
		t.Fatal("invalid SYSTEM_INFO layout")
	}
	info, err := (Native{}).System(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Hostname == "" || info.Windows.Build == 0 || info.Architecture == "unknown" || info.ProcessArchitecture != runtime.GOARCH {
		t.Fatal("incomplete native system data")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = (Native{}).System(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
}
