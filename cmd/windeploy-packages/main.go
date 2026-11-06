// Command windeploy-packages plans and installs catalog software from the command
// line. It is intended for controlled VM testing and headless workstation setup.
package main

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"windeploy/core/inventory"
	"windeploy/core/packages"
)

func main() {
	adapters := cliAdapters{
		facts:     packages.WindowsFacts{},
		fetcher:   packages.NewHTTPFetcher(),
		runner:    packages.WindowsRunner{},
		signer:    packages.WindowsSigner{},
		elevation: packages.WindowsElevation{},
		winget:    &packages.WindowsWinget{},
	}
	if cache, err := os.UserCacheDir(); err == nil {
		adapters.cacheDir = filepath.Join(cache, "WinDeploy", "Downloads")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	adapters.system, adapters.systemErr = inventory.Native{}.System(ctx)
	cancel()
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, adapters))
}
