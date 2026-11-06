package inventory

import (
	"context"
	"fmt"
	"strings"
)

type CPUInfo struct {
	Model             string `json:"model"`
	Vendor            string `json:"vendor,omitempty"`
	LogicalProcessors uint32 `json:"logical_processors"`
}
type CPUProvider interface {
	CPU(context.Context) (CPUInfo, error)
}

func cpuInfo(model, vendor string, count uint32) (CPUInfo, error) {
	result := CPUInfo{Model: strings.TrimSpace(model), Vendor: strings.TrimSpace(vendor), LogicalProcessors: count}
	if result.Model == "" || count == 0 {
		return result, fmt.Errorf("incomplete CPU information")
	}
	return result, nil
}
