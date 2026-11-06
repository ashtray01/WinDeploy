//go:build windows

package inventory

import (
	"context"
	"fmt"

	"golang.org/x/sys/windows"
)

func (Native) Volumes(ctx context.Context) ([]Volume, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	buffer := make([]uint16, 256)
	for tries := 0; tries < 4; tries++ {
		n, err := windows.GetLogicalDriveStrings(uint32(len(buffer)), &buffer[0])
		if err != nil {
			return nil, fmt.Errorf("enumerate volume roots failed")
		}
		if n >= uint32(len(buffer)) {
			if n > 65535 {
				return nil, fmt.Errorf("volume buffer exceeds limit")
			}
			buffer = make([]uint16, n+1)
			continue
		}
		roots, err := driveRoots(buffer[:n+1])
		if err != nil {
			return nil, err
		}
		volumes := make([]Volume, 0, len(roots))
		for _, root := range roots {
			ptr, _ := windows.UTF16PtrFromString(root)
			kind := "unknown"
			switch windows.GetDriveType(ptr) {
			case windows.DRIVE_FIXED:
				kind = "fixed"
			case windows.DRIVE_RAMDISK:
				kind = "ramdisk"
			case windows.DRIVE_REMOVABLE:
				kind = "removable"
			case windows.DRIVE_REMOTE:
				kind = "network"
			case windows.DRIVE_CDROM:
				kind = "optical"
			}
			volumes = append(volumes, Volume{Root: root, Kind: kind})
		}
		return collectVolumes(ctx, volumes, func(root string) (Volume, error) {
			ptr, _ := windows.UTF16PtrFromString(root)
			var available, total, free uint64
			err := windows.GetDiskFreeSpaceEx(ptr, &available, &total, &free)
			return Volume{TotalBytes: total, FreeBytes: free, AvailableBytes: available}, err
		})
	}
	return nil, fmt.Errorf("volume list changed repeatedly")
}
