//go:build !windows

package packages

import (
	"context"
	"errors"

	"windeploy/core/inventory"
)

// ErrUnsupportedPlatform is returned by native adapters outside Windows.
var ErrUnsupportedPlatform = errors.New("package operations are supported on Windows only")

// UnsupportedFacts keeps the core package buildable for cross-platform tests and
// documentation tooling. It never reports installed software.
type UnsupportedFacts struct{}

func (UnsupportedFacts) Apps(context.Context) ([]inventory.InstalledApp, error) {
	return nil, ErrUnsupportedPlatform
}
func (UnsupportedFacts) File(context.Context, string) (FileInfo, error) {
	return FileInfo{}, ErrUnsupportedPlatform
}
func (UnsupportedFacts) ExpandPath(path string) string { return path }

// UnsupportedRunner refuses to execute processes.
type UnsupportedRunner struct{}

func (UnsupportedRunner) Run(context.Context, Command, func(string)) (Execution, error) {
	return Execution{}, ErrUnsupportedPlatform
}

// UnsupportedSigner refuses to verify signatures.
type UnsupportedSigner struct{}

func (UnsupportedSigner) Verify(context.Context, string) (Signature, error) {
	return Signature{}, ErrUnsupportedPlatform
}

// UnsupportedElevation treats every process as non-elevated.
type UnsupportedElevation struct{}

func (UnsupportedElevation) Elevated(context.Context) (bool, error) {
	return false, ErrUnsupportedPlatform
}

// UnsupportedWinget reports that the Windows Package Manager is unavailable.
type UnsupportedWinget struct{}

func (UnsupportedWinget) Available(context.Context) (bool, string) {
	return false, ""
}

// The Windows-prefixed names keep shared callers (CLI, legacy front-ends) buildable
// on other platforms; every operation fails closed at runtime.
type WindowsFacts = UnsupportedFacts
type WindowsRunner = UnsupportedRunner
type WindowsSigner = UnsupportedSigner
type WindowsElevation = UnsupportedElevation
type WindowsWinget = UnsupportedWinget
