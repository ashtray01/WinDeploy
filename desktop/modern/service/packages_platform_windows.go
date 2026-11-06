//go:build windows

package service

import (
	"os"
	"path/filepath"

	"windeploy/core/packages"
)

// packagePlatform supplies the real Windows adapters. Tests can replace the
// manager dependencies through the service constructor in unit tests.
func packagePlatform() (packages.Facts, packages.Runner, packages.Signer, packages.Elevation, packages.WingetProbe) {
	return packages.WindowsFacts{}, packages.WindowsRunner{}, packages.WindowsSigner{}, packages.WindowsElevation{}, &packages.WindowsWinget{}
}

func packageCacheDirectory() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "WinDeploy", "Downloads"), nil
}
