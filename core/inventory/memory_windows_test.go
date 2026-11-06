//go:build windows

package inventory

import (
	"context"
	"testing"
	"unsafe"
)

func TestNativeMemoryReadOnly(t *testing.T) {
	if unsafe.Sizeof(memoryStatus{}) != 64 {
		t.Fatal("invalid MEMORYSTATUSEX layout")
	}
	info, err := (Native{}).Memory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.TotalPhysicalBytes == 0 || info.AvailablePhysicalBytes > info.TotalPhysicalBytes {
		t.Fatal("invalid physical memory")
	}
}
