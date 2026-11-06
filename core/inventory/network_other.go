//go:build !windows

package inventory

import "context"

func (Native) NetworkConfig(ctx context.Context) ([]AdapterConfig, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, ErrUnsupported
}
