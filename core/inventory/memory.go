package inventory

import (
	"context"
	"fmt"
)

type MemoryInfo struct {
	TotalPhysicalBytes     uint64 `json:"total_physical_bytes"`
	AvailablePhysicalBytes uint64 `json:"available_physical_bytes"`
	LoadPercent            uint32 `json:"load_percent"`
}
type MemoryProvider interface {
	Memory(context.Context) (MemoryInfo, error)
}

func memoryInfo(total, available uint64, load uint32) (MemoryInfo, error) {
	result := MemoryInfo{TotalPhysicalBytes: total, AvailablePhysicalBytes: available, LoadPercent: load}
	if total == 0 || available > total || load > 100 {
		return result, fmt.Errorf("invalid physical memory information")
	}
	return result, nil
}
