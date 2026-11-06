package inventory

import (
	"context"
	"fmt"
	"sort"
	"unicode/utf16"
)

type Volume struct {
	Root           string `json:"root"`
	Kind           string `json:"kind"`
	TotalBytes     uint64 `json:"total_bytes"`
	FreeBytes      uint64 `json:"free_bytes"`
	AvailableBytes uint64 `json:"available_bytes"`
	CapacityKnown  bool   `json:"capacity_known"`
	Issue          string `json:"issue,omitempty"`
}
type VolumeProvider interface {
	Volumes(context.Context) ([]Volume, error)
}

func driveRoots(buffer []uint16) ([]string, error) {
	roots := []string{}
	start := 0
	for i, value := range buffer {
		if value != 0 {
			continue
		}
		if i == start {
			return roots, nil
		}
		root := string(utf16.Decode(buffer[start:i]))
		if len(root) != 3 || root[1:] != ":\\" {
			return nil, fmt.Errorf("invalid volume root")
		}
		roots = append(roots, root)
		start = i + 1
	}
	return nil, fmt.Errorf("unterminated volume list")
}

func collectVolumes(ctx context.Context, volumes []Volume, space func(string) (Volume, error)) ([]Volume, error) {
	result := make([]Volume, 0, len(volumes))
	failed := false
	for _, volume := range volumes {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if volume.Kind != "fixed" && volume.Kind != "ramdisk" {
			volume.Issue = "capacity not queried for non-fixed drive"
		} else {
			capacity, err := space(volume.Root)
			if err != nil {
				volume.Issue = "read volume capacity failed"
				failed = true
			} else if capacity.TotalBytes == 0 || capacity.AvailableBytes > capacity.TotalBytes {
				volume.Issue = "invalid volume capacity"
				failed = true
			} else {
				volume.TotalBytes = capacity.TotalBytes
				volume.FreeBytes = capacity.FreeBytes
				volume.AvailableBytes = capacity.AvailableBytes
				volume.CapacityKnown = true
			}
		}
		result = append(result, volume)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Root < result[j].Root })
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if failed {
		return result, fmt.Errorf("some volume capacities are unavailable")
	}
	return result, nil
}
