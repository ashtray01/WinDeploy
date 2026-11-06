package inventory

import (
	"context"
	"errors"
	"time"
)

var ErrIncomplete = errors.New("inventory contains incomplete sections")

type Provider interface {
	SystemProvider
	CPUProvider
	MemoryProvider
	VolumeProvider
	InterfaceProvider
	NetworkProvider
	AppProvider
}

type Section struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

// Report distinguishes an empty successful section from a failed/missing one.
// It contains private local machine metadata and is not safe for public sharing.
type Report struct {
	Version    int             `json:"version"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt time.Time       `json:"finished_at"`
	Complete   bool            `json:"complete"`
	Sections   []Section       `json:"sections"`
	System     *SystemInfo     `json:"system,omitempty"`
	CPU        *CPUInfo        `json:"cpu,omitempty"`
	Memory     *MemoryInfo     `json:"memory,omitempty"`
	Volumes    []Volume        `json:"volumes"`
	Interfaces []InterfaceInfo `json:"interfaces"`
	Network    []AdapterConfig `json:"network"`
	Apps       []InstalledApp  `json:"apps"`
}

// Collect reads each section once in order, preserving usable partial data.
// A failed source does not stop independent reads; cancellation does. Raw provider
// errors never enter the report. ErrIncomplete must not be treated as success.
func Collect(ctx context.Context, provider Provider) (Report, error) {
	report := Report{Version: 1, StartedAt: time.Now().UTC(), Sections: []Section{}, Volumes: []Volume{}, Interfaces: []InterfaceInfo{}, Network: []AdapterConfig{}, Apps: []InstalledApp{}}
	if provider == nil {
		report.FinishedAt = time.Now().UTC()
		return report, errors.New("inventory provider is required")
	}
	type step struct {
		name string
		read func() (bool, error)
	}
	steps := []step{
		{"system", func() (bool, error) {
			v, e := provider.System(ctx)
			has := v.Hostname != "" || v.Windows.Major != 0
			if e == nil || has {
				report.System = &v
			}
			return has, e
		}},
		{"cpu", func() (bool, error) {
			v, e := provider.CPU(ctx)
			has := v.Model != "" || v.LogicalProcessors != 0
			if e == nil || has {
				report.CPU = &v
			}
			return has, e
		}},
		{"memory", func() (bool, error) {
			v, e := provider.Memory(ctx)
			has := v.TotalPhysicalBytes != 0
			if e == nil || has {
				report.Memory = &v
			}
			return has, e
		}},
		{"volumes", func() (bool, error) {
			v, e := provider.Volumes(ctx)
			if v != nil {
				report.Volumes = v
			}
			return len(v) > 0, e
		}},
		{"interfaces", func() (bool, error) {
			v, e := provider.Interfaces(ctx)
			if v != nil {
				report.Interfaces = v
			}
			return len(v) > 0, e
		}},
		{"network", func() (bool, error) {
			v, e := provider.NetworkConfig(ctx)
			if v != nil {
				report.Network = v
			}
			return len(v) > 0, e
		}},
		{"apps", func() (bool, error) {
			v, e := provider.Apps(ctx)
			if v != nil {
				report.Apps = v
			}
			return len(v) > 0, e
		}},
	}
	var stopped error
	incomplete := false
	for _, step := range steps {
		if stopped == nil {
			stopped = ctx.Err()
		}
		if stopped != nil {
			report.Sections = append(report.Sections, Section{Name: step.name, Status: "not-collected", Error: "cancelled"})
			continue
		}
		has, err := step.read()
		status, code := "ok", ""
		if ctx.Err() != nil {
			err = ctx.Err()
		}
		if err != nil {
			incomplete = true
			status, code = "failed", "read_failed"
			if has {
				status = "partial"
			}
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				stopped = err
				status, code = "cancelled", "cancelled"
			}
		}
		report.Sections = append(report.Sections, Section{Name: step.name, Status: status, Error: code})
	}
	report.FinishedAt = time.Now().UTC()
	if stopped != nil {
		return report, stopped
	}
	if incomplete {
		return report, ErrIncomplete
	}
	report.Complete = true
	return report, nil
}
