// Command windeploy-run executes profile components headlessly, persists resumable
// sessions and writes HTML/JSON reports. Intended for controlled VM runs.
package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"

	"windeploy/core/inventory"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cache, _ := os.UserCacheDir()
	base := filepath.Join(cache, "WinDeploy")
	system := readSystem(ctx)
	environment, err := buildEnvironment(base, system)
	if err != nil {
		os.Stderr.WriteString("WinDeploy не смог подготовить компоненты: " + err.Error() + "\n")
		os.Exit(1)
	}
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr, environment)
	os.Exit(code)
}

func readSystem(ctx context.Context) inventory.SystemInfo {
	system, err := inventory.Native{}.System(ctx)
	if err != nil {
		return inventory.SystemInfo{}
	}
	return system
}
