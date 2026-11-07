//go:build windows

package inventory

import (
	"context"
	"encoding/hex"
	"fmt"
	"unsafe"

	"golang.org/x/sys/windows"
)

// BootID reads the kernel boot GUID on Modern Windows. This private NT query is
// isolated here and fails closed on an unavailable API or changed response size.
// Layout: GUID, FIRMWARE_TYPE, padding, ULONGLONG BootFlags (32 bytes).
// Reference: System Informer phnt/include/ntexapi.h, SYSTEM_BOOT_ENVIRONMENT_INFORMATION.
func (Native) BootID(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	if windows.RtlGetVersion().MajorVersion < 10 {
		return "", fmt.Errorf("boot identity requires Modern Windows")
	}
	var info struct {
		ID       windows.GUID
		Firmware uint32
		Padding  uint32
		Flags    uint64
	}
	var returned uint32
	err := windows.NtQuerySystemInformation(90, unsafe.Pointer(&info), uint32(unsafe.Sizeof(info)), &returned)
	if err != nil || returned != uint32(unsafe.Sizeof(info)) || info.ID == (windows.GUID{}) {
		return "", fmt.Errorf("kernel boot identity unavailable")
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	bytes := (*[16]byte)(unsafe.Pointer(&info.ID))
	return hex.EncodeToString(bytes[:]), nil
}
