//go:build windows

package inventory

import (
	"context"
	"testing"
)

func TestNativeVolumesReadOnly(t *testing.T) {
	volumes, err := (Native{}).Volumes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(volumes) == 0 {
		t.Fatal("no volume roots")
	}
	known := false
	for _, v := range volumes {
		if v.CapacityKnown && v.TotalBytes > 0 {
			known = true
		}
	}
	if !known {
		t.Fatal("no readable local capacity")
	}
}
