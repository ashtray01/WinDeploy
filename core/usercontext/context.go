// Package usercontext gates operations that use HKCU, Desktop or credentials.
package usercontext

import (
	"context"
	"errors"
)

var ErrUnconfirmed = errors.New("user context is not confirmed; run WinDeploy as the interactive desktop user")

// Identity uses SIDs from native tokens, never environment names or argv.
type Identity struct{ ProcessSID, ShellSID string }

type Reader func(context.Context) (Identity, error)

func Check(ctx context.Context, read Reader) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if read == nil {
		return ErrUnconfirmed
	}
	identity, err := read(ctx)
	if err != nil || identity.ProcessSID == "" || identity.ShellSID == "" || identity.ProcessSID != identity.ShellSID {
		return ErrUnconfirmed
	}
	return ctx.Err()
}

type Native struct{}

func (Native) Check(ctx context.Context) error { return Check(ctx, (Native{}).Read) }
