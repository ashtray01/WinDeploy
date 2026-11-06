//go:build windows

package inventory

import (
	"context"
	"fmt"
	"unsafe"
)

var globalMemoryStatusEx = kernel32.NewProc("GlobalMemoryStatusEx")

type memoryStatus struct {
	Length                   uint32
	Load                     uint32
	TotalPhysical            uint64
	AvailablePhysical        uint64
	TotalPageFile            uint64
	AvailablePageFile        uint64
	TotalVirtual             uint64
	AvailableVirtual         uint64
	AvailableExtendedVirtual uint64
}

func (Native) Memory(ctx context.Context) (MemoryInfo, error) {
	if err := ctx.Err(); err != nil {
		return MemoryInfo{}, err
	}
	if err := globalMemoryStatusEx.Find(); err != nil {
		return MemoryInfo{}, fmt.Errorf("memory API unavailable")
	}
	status := memoryStatus{Length: uint32(unsafe.Sizeof(memoryStatus{}))}
	ok, _, _ := globalMemoryStatusEx.Call(uintptr(unsafe.Pointer(&status)))
	if err := ctx.Err(); err != nil {
		return MemoryInfo{}, err
	}
	if ok == 0 {
		return MemoryInfo{}, fmt.Errorf("read physical memory failed")
	}
	return memoryInfo(status.TotalPhysical, status.AvailablePhysical, status.Load)
}
