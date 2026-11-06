//go:build !windows

package inventory

import "context"

func (Native) Memory(ctx context.Context) (MemoryInfo, error) {
	if err := ctx.Err(); err != nil {
		return MemoryInfo{}, err
	}
	return MemoryInfo{}, ErrUnsupported
}
