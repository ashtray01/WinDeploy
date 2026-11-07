// Package buildinfo carries the release identity injected by the build script.
// It is deliberately independent of the UI so the CLI, the Wails service and the
// Windows version resource can expose the same release number.
package buildinfo

import "strings"

// Version is a semantic version such as 0.1.0-beta.1. Release builds replace the
// development default with -ldflags "-X windeploy/internal/buildinfo.Version=...".
var Version = "0.0.0-dev"

// Commit is the full or short Git revision used for diagnostics. It is not a
// security boundary and must never contain local paths or credentials.
var Commit = "dev"

const ProductName = "WinDeploy"

// DisplayVersion returns the version shown in the UI and release metadata.
func DisplayVersion() string {
	value := strings.TrimSpace(Version)
	if value == "" {
		return "0.0.0-dev"
	}
	return value
}

// DisplayCommit returns a short, log-safe revision label.
func DisplayCommit() string {
	value := strings.TrimSpace(Commit)
	if value == "" {
		return "dev"
	}
	if len(value) > 12 {
		return value[:12]
	}
	return value
}
