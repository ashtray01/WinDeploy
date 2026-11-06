//go:build windows

package inventory

import (
	"context"
	"errors"
	"testing"
)

func TestNativeCPUReadOnly(t *testing.T) {
	info, err := (Native{}).CPU(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.Model == "" || info.LogicalProcessors == 0 {
		t.Fatal("incomplete CPU")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = (Native{}).CPU(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
