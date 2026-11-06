//go:build !windows

package inventory

import "context"

func (Native) CPU(ctx context.Context) (CPUInfo, error) {
	if err := ctx.Err(); err != nil {
		return CPUInfo{}, err
	}
	return CPUInfo{}, ErrUnsupported
}
