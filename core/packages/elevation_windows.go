//go:build windows

package packages

import (
	"context"
	"os"

	"golang.org/x/sys/windows"
)

// WindowsElevation checks the current process token. It never requests elevation.
type WindowsElevation struct{}

// Elevated implements Elevation.
func (WindowsElevation) Elevated(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false, err
	}
	defer token.Close()
	return token.IsElevated(), nil
}

// RelaunchElevated starts the current executable through the UAC prompt. The
// caller must obtain explicit user consent first; this function only performs the
// requested elevation and never bypasses UAC.
func RelaunchElevated(arguments string) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	path16, err := windows.UTF16PtrFromString(executable)
	if err != nil {
		return err
	}
	args16, err := windows.UTF16PtrFromString(arguments)
	if err != nil {
		return err
	}
	verb16, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb16, path16, args16, nil, windows.SW_SHOWNORMAL)
}
