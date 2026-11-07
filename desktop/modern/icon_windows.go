package main

import (
	_ "embed"
	"time"

	"github.com/wailsapp/wails/v3/pkg/w32"
	"golang.org/x/sys/windows"
)

// mainWindowClass names the native window so the embedded icon can be installed
// after Wails registers its class with the default system icon. The same class is
// passed to WindowsOptions.WndClass and is also compiled into the EXE resources.
const mainWindowClass = "WinDeployMainWindow"

// windowIcon is the multi-size ICO that is embedded in the EXE as resource ID 1.
//
//go:embed assets/windows/icon.ico
var windowIcon []byte

// applyWindowIcon installs the large and small window icons on the existing native
// window. Windows keeps the icon handles for the window lifetime. Failure only
// leaves the default icon; it never blocks startup.
func applyWindowIcon() {
	go func() {
		className, err := windows.UTF16PtrFromString(mainWindowClass)
		if err != nil {
			return
		}
		deadline := time.Now().Add(20 * time.Second)
		for time.Now().Before(deadline) {
			if hwnd := w32.FindWindowW(className, nil); hwnd != 0 {
				if !setWindowIcons(hwnd) {
					logStation("иконка окна не применена")
				}
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		logStation("окно не найдено; иконка окна не применена")
	}()
}

func setWindowIcons(hwnd w32.HWND) bool {
	large, err := w32.CreateLargeHIconFromImage(windowIcon)
	if err != nil || large == 0 {
		return false
	}
	small, err := w32.CreateSmallHIconFromImage(windowIcon)
	if err != nil || small == 0 {
		return false
	}
	w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_BIG, uintptr(large))
	w32.SendMessage(hwnd, w32.WM_SETICON, w32.ICON_SMALL, uintptr(small))
	return true
}
