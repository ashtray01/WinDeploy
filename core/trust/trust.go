// Package trust verifies that local state files live in storage that other local
// principals cannot rewrite. It is a partial boundary: it checks the owner and the
// write access control entries of the file and its directory. It is not an
// authentication system, does not prove who created the file and cannot stop an
// administrator who can take ownership. Callers must still treat unreadable state
// as untrusted and must never perform system changes after a trust failure.
package trust

import (
	"context"
	"errors"
)

// ErrUntrusted reports storage that cannot be used for trusted local state.
var ErrUntrusted = errors.New("local storage is not trusted: it must be owned by the current user and not writable by others")

// Check verifies an existing path. dir selects directory expectations. Unknown,
// unreadable or replaced (reparse point) paths fail closed with ErrUntrusted.
func Check(ctx context.Context, path string, dir bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if path == "" {
		return ErrUntrusted
	}
	if err := check(path, dir); err != nil {
		return ErrUntrusted
	}
	return ctx.Err()
}
