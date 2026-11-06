//go:build !windows

package inventory

import "context"

func (Native) System(ctx context.Context) (SystemInfo, error) {
	if err := ctx.Err(); err != nil {
		return SystemInfo{}, err
	}
	return SystemInfo{}, ErrUnsupported
}
