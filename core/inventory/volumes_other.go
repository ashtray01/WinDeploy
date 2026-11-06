//go:build !windows

package inventory

import "context"

func (Native) Volumes(ctx context.Context) ([]Volume, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, ErrUnsupported
}
