package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/lxn/walk"
	profilecatalog "windeploy/catalog"
	"windeploy/core/inventory"
	"windeploy/core/packages"
)

func hasFlag(name string) bool {
	for _, arg := range os.Args[1:] {
		if arg == name {
			return true
		}
	}
	return false
}

// runSelfTest exercises the core paths used by the GUI without creating a
// window. It is intended for CI and Win7 smoke runs.
func runSelfTest() int {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	system, err := inventory.Native{}.System(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "system:", err)
		return 1
	}
	catalog, err := profilecatalog.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "catalog:", err)
		return 1
	}
	if len(catalog.Packages) == 0 {
		fmt.Fprintln(os.Stderr, "catalog is empty")
		return 1
	}
	pkg, ok := catalog.Find("sevenzip")
	if !ok {
		fmt.Fprintln(os.Stderr, "sevenzip is missing")
		return 1
	}
	state, err := packages.Detect(ctx, packages.WindowsFacts{}, pkg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "detect:", err)
		return 1
	}
	compatible, err := inventory.Applicable(pkg.Requirements, system)
	if err != nil {
		fmt.Fprintln(os.Stderr, "compatibility:", err)
		return 1
	}
	fmt.Printf("LEGACY_OK hostname=%s windows=%s arch=%s packages=%d sevenzip_installed=%t compatible=%t\n", system.Hostname, windowsFriendly(system.Windows), system.Architecture, len(catalog.Packages), state.Installed, compatible)
	return 0
}

func runSmoke(app *legacyApp) {
	app.uiReady.Store(true)
	timer := time.AfterFunc(3*time.Second, func() {
		app.window.Synchronize(func() { _ = app.window.Close() })
	})
	app.window.Run()
	timer.Stop()
	fmt.Println("LEGACY_GUI_SMOKE_OK")
}

func showFatal(message string) {
	walk.MsgBox(nil, "WinDeploy Legacy", message, walk.MsgBoxIconError)
}
