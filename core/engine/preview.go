package engine

import (
	"fmt"
	"time"
)

// PlanPreview is structural validation only. It does not check OS applicability,
// installed software, permissions or sources and never calls an Adapter.
type PlanPreview struct {
	Scope string        `json:"scope"`
	Tasks []PreviewTask `json:"tasks"`
}

// PreviewTask intentionally excludes parameters, execution status and results.
type PreviewTask struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Category     string         `json:"category"`
	Dependencies []string       `json:"dependencies,omitempty"`
	Requirements OSRequirements `json:"requirements"`
	Timeout      time.Duration  `json:"timeout"`
	ResourceKey  string         `json:"resource_key,omitempty"`
}

func pendingPlan(tasks []Task) ([]Task, error) {
	ordered, err := OrderTasks(tasks)
	if err != nil {
		return nil, err
	}
	for _, task := range ordered {
		if task.Status != Pending {
			return nil, fmt.Errorf("task %q must be Pending; resume is not supported", task.ID)
		}
	}
	return ordered, nil
}

// PreviewPlan returns an independent copy of safe plan metadata in execution order.
func PreviewPlan(tasks []Task) (PlanPreview, error) {
	ordered, err := pendingPlan(tasks)
	if err != nil {
		return PlanPreview{}, err
	}
	preview := PlanPreview{Scope: "structure-and-dag-only", Tasks: make([]PreviewTask, 0, len(ordered))}
	for _, task := range ordered {
		preview.Tasks = append(preview.Tasks, PreviewTask{
			ID: task.ID, Name: task.Name, Category: task.Category, Timeout: task.Timeout, ResourceKey: task.ResourceKey,
			Dependencies: append([]string(nil), task.Dependencies...),
			Requirements: OSRequirements{
				Versions:      append([]string(nil), task.Requirements.Versions...),
				Architectures: append([]string(nil), task.Requirements.Architectures...),
			},
		})
	}
	return preview, nil
}
