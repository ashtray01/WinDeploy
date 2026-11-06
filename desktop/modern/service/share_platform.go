package service

import "windeploy/core/shares"

// sharePlatform returns the native adapters. On non-Windows builds the same
// types exist but operations fail closed.
func sharePlatform() (shares.CredentialStore, shares.Connector, shares.ShortcutCreator) {
	return shares.WindowsCredentialStore{}, shares.WindowsConnector{}, shares.WindowsShortcut{}
}

func desktopDirectory() (string, error) { return shares.DesktopDirectory() }
