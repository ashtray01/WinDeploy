//go:build windows

package inventory

import (
	"context"
	"fmt"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// CPU returns the first processor's brand/vendor and active logical processors
// across all groups. It does not claim to inventory heterogeneous CPU packages.
func (Native) CPU(ctx context.Context) (CPUInfo, error) {
	if err := ctx.Err(); err != nil {
		return CPUInfo{}, err
	}
	count := windows.GetActiveProcessorCount(0xffff)
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, registry.QUERY_VALUE)
	if err != nil {
		return CPUInfo{LogicalProcessors: count}, fmt.Errorf("read CPU registry failed")
	}
	defer key.Close()
	model, _, err := key.GetStringValue("ProcessorNameString")
	if err != nil {
		return CPUInfo{LogicalProcessors: count}, fmt.Errorf("read CPU model failed")
	}
	vendor, _, _ := key.GetStringValue("VendorIdentifier")
	if err = ctx.Err(); err != nil {
		return CPUInfo{}, err
	}
	return cpuInfo(model, vendor, count)
}
