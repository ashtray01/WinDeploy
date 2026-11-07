//go:build !windows

package inventory

import (
	"context"
	"fmt"
)

func (Native) BootID(context.Context) (string, error) {
	return "", fmt.Errorf("kernel boot identity unavailable")
}
