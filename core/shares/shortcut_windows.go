//go:build windows

package shares

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func initializeShortcutCOM() (func(), error) {
	err := windows.CoInitializeEx(0, coinitApartment)
	if err == nil || err == syscall.Errno(1) {
		return windows.CoUninitialize, nil
	} // S_OK / S_FALSE both need Uninitialize.
	if err == syscall.Errno(0x80010106) {
		return func() {}, nil
	} // Existing apartment (RPC_E_CHANGED_MODE).
	return nil, fmt.Errorf("shortcut COM initialization failed")
}

// Matches loads only the local .lnk, without IShellLink.Resolve or target access.
func (WindowsShortcut) Matches(ctx context.Context, spec ShortcutSpec) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if _, err := os.Stat(spec.Path); os.IsNotExist(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	uninit, err := initializeShortcutCOM()
	if err != nil {
		return false, err
	}
	defer uninit()
	var link unsafe.Pointer
	hr, _, _ := procCoCreate.Call(uintptr(unsafe.Pointer(&clsidShellLink)), 0, clsctxInprocServer, uintptr(unsafe.Pointer(&iidIShellLinkW)), uintptr(unsafe.Pointer(&link)))
	if failedHR(hr) || link == nil {
		return false, fmt.Errorf("shortcut reader unavailable")
	}
	defer comCall(link, 2)
	var persist unsafe.Pointer
	if failedHR(comCall(link, 0, uintptr(unsafe.Pointer(&iidIPersistFile)), uintptr(unsafe.Pointer(&persist)))) || persist == nil {
		return false, fmt.Errorf("shortcut persistence unavailable")
	}
	defer comCall(persist, 2)
	path, err := windows.UTF16PtrFromString(spec.Path)
	if err != nil {
		return false, err
	}
	if failedHR(comCall(persist, 5, uintptr(unsafe.Pointer(path)), 0)) {
		return false, fmt.Errorf("shortcut could not be loaded")
	}
	var target [260]uint16
	if comCall(link, 3, uintptr(unsafe.Pointer(&target[0])), uintptr(len(target)), 0, 4) != 0 {
		return false, fmt.Errorf("shortcut target unavailable")
	}
	var arguments [32768]uint16
	if failedHR(comCall(link, 10, uintptr(unsafe.Pointer(&arguments[0])), uintptr(len(arguments)))) {
		return false, fmt.Errorf("shortcut arguments unavailable")
	}
	if err := ctx.Err(); err != nil {
		return false, err
	}
	return strings.EqualFold(filepath.Clean(windows.UTF16ToString(target[:])), filepath.Clean(spec.Target)) && windows.UTF16ToString(arguments[:]) == spec.Arguments, nil
}
