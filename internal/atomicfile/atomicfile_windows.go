//go:build windows

package atomicfile

import (
	"errors"

	"golang.org/x/sys/windows"
)

// isTransient reports sharing conflicts with a file that is still open elsewhere.
// Antivirus, search indexers and backup agents hold the previous version briefly
// while scanning it, which makes MoveFileEx report the conflict as access denied.
func isTransient(err error) bool {
	return errors.Is(err, windows.ERROR_ACCESS_DENIED) ||
		errors.Is(err, windows.ERROR_SHARING_VIOLATION) ||
		errors.Is(err, windows.ERROR_LOCK_VIOLATION)
}
