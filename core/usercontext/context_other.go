//go:build !windows

package usercontext

import "context"

func (Native) Read(context.Context) (Identity, error) { return Identity{}, ErrUnconfirmed }
