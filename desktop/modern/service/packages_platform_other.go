//go:build !windows

package service

import (
	"os"
	"path/filepath"

	"windeploy/core/packages"
)

func packagePlatform() (packages.Facts, packages.Runner, packages.Signer, packages.Elevation, packages.WingetProbe) {
	return packages.UnsupportedFacts{}, packages.UnsupportedRunner{}, packages.UnsupportedSigner{}, packages.UnsupportedElevation{}, packages.UnsupportedWinget{}
}

func packageCacheDirectory() (string, error) {
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cache, "WinDeploy", "Downloads"), nil
}
