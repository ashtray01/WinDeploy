package profiles

import (
	"fmt"

	"windeploy/core/engine"
)

// Clone returns an independently editable profile with a new identity. Parent
// references remain intact; Resolve first if a standalone flattened clone is wanted.
func Clone(profile Profile, id, name string) (Profile, error) {
	if err := profile.Validate(); err != nil {
		return Profile{}, err
	}
	result := copyProfile(profile)
	result.ID = id
	result.Name = name
	if err := result.Validate(); err != nil {
		return Profile{}, err
	}
	return result, nil
}

// BuildPlan converts a resolved profile to isolated Pending tasks. Nil selection
// means all enabled components; a non-nil empty selection means none. Explicit
// selection includes the transitive dependency closure, never silently ignores a
// disabled/missing dependency, and returns stable OrderTasks order. This checks
// metadata/DAG only: it does not validate actual package versions or sources.
func BuildPlan(profile Profile, selection []string) ([]engine.Task, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}
	if profile.Extends != "" {
		return nil, fmt.Errorf("resolve profile inheritance before building a plan")
	}
	components := make(map[string]Component, len(profile.Components))
	for _, c := range profile.Components {
		components[c.ID] = c
	}
	roots := selection
	if roots == nil {
		roots = make([]string, 0, len(profile.Components))
		for _, c := range profile.Components {
			if !c.Disabled {
				roots = append(roots, c.ID)
			}
		}
	}
	seenRoots := map[string]bool{}
	for _, id := range roots {
		if seenRoots[id] {
			return nil, fmt.Errorf("duplicate selected component")
		}
		seenRoots[id] = true
	}
	included := map[string]bool{}
	queue := append([]string{}, roots...)
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		if included[id] {
			continue
		}
		c, exists := components[id]
		if !exists {
			return nil, fmt.Errorf("selected component or dependency not found")
		}
		if c.Disabled {
			return nil, fmt.Errorf("selected component or dependency is disabled")
		}
		included[id] = true
		queue = append(queue, c.Dependencies...)
	}
	tasks := make([]engine.Task, 0, len(included))
	for _, c := range profile.Components {
		if !included[c.ID] {
			continue
		}
		task, err := copyComponent(c).task()
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return engine.OrderTasks(tasks)
}

// Preview produces the same structural-only preview as the engine, without
// creating an adapter or exposing component parameters.
func Preview(profile Profile, selection []string) (engine.PlanPreview, error) {
	tasks, err := BuildPlan(profile, selection)
	if err != nil {
		return engine.PlanPreview{}, err
	}
	return engine.PreviewPlan(tasks)
}
