//go:build !windows

package inventory

import "context"

func (Native) Apps(ctx context.Context) ([]InstalledApp, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, ErrUnsupported
}
