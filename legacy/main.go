// Command windeploy-legacy is the native Windows 7 SP1-compatible front-end.
// It uses the common Go core and the Walk Win32 toolkit; no WebView2 is required.
package main

import (
	"fmt"
	"os"
)

func main() {
	if hasFlag("--selftest") {
		os.Exit(runSelfTest())
	}
	smoke := hasFlag("--smoke")
	app, err := newLegacyApp()
	if err != nil {
		if smoke {
			fmt.Fprintln(os.Stderr, "LEGACY_GUI_SMOKE_FAILED:", err)
			os.Exit(2)
		}
		showFatal("Не удалось запустить WinDeploy Legacy.\r\n\r\n" + err.Error())
		return
	}
	if smoke {
		runSmoke(app)
		return
	}
	app.uiReady.Store(true)
	app.window.Run()
}
