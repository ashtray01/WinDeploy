//go:build windows

package inventory

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/windows"
)

var kernel32 = windows.NewLazySystemDLL("kernel32.dll")
var getNativeSystemInfo = kernel32.NewProc("GetNativeSystemInfo")

// SYSTEM_INFO uses pointer-sized members; its layout is checked on 386/amd64.
type nativeSystemInfo struct {
	Architecture          uint16
	Reserved              uint16
	PageSize              uint32
	MinimumAddress        uintptr
	MaximumAddress        uintptr
	ActiveMask            uintptr
	ProcessorCount        uint32
	ProcessorType         uint32
	AllocationGranularity uint32
	ProcessorLevel        uint16
	ProcessorRevision     uint16
}

func (Native) System(ctx context.Context) (SystemInfo, error) {
	if err := ctx.Err(); err != nil {
		return SystemInfo{}, err
	}
	host, err := os.Hostname()
	if err != nil {
		return SystemInfo{}, fmt.Errorf("read hostname failed")
	}
	if err = getNativeSystemInfo.Find(); err != nil {
		return SystemInfo{}, fmt.Errorf("native system API unavailable")
	}
	var system nativeSystemInfo
	getNativeSystemInfo.Call(uintptr(unsafe.Pointer(&system)))
	version := windows.RtlGetVersion()
	if version.MajorVersion == 0 || version.BuildNumber == 0 {
		return SystemInfo{}, fmt.Errorf("Windows version unavailable")
	}
	if err = ctx.Err(); err != nil {
		return SystemInfo{}, err
	}
	return SystemInfo{Hostname: host, Architecture: architecture(system.Architecture), ProcessArchitecture: runtime.GOARCH, Windows: Version{Major: version.MajorVersion, Minor: version.MinorVersion, Build: version.BuildNumber, ServicePackMajor: version.ServicePackMajor, ProductType: version.ProductType}}, nil
}
