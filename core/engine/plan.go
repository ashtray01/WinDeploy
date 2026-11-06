package engine

import "fmt"

// OrderTasks validates a plan and returns its tasks in dependency order.
// Among tasks whose dependencies have already been emitted, the task with the
// lowest original index is chosen first. An empty plan is valid.
//
// On error no partial plan is returned. The input is never modified. The output
// is a new slice of shallow Task copies; nested maps and slices remain shared.
// OS compatibility and status transitions are outside this validation's scope.
func OrderTasks(tasks []Task) ([]Task, error) {
	indices := make(map[string]int, len(tasks))
	for i, task := range tasks {
		if err := task.Validate(); err != nil {
			return nil, fmt.Errorf("task at index %d (%q): %w", i, task.ID, err)
		}
		if _, exists := indices[task.ID]; exists {
			return nil, fmt.Errorf("duplicate task ID %q", task.ID)
		}
		indices[task.ID] = i
	}

	remaining := make([]int, len(tasks))
	dependents := make([][]int, len(tasks))
	for i, task := range tasks {
		remaining[i] = len(task.Dependencies)
		for _, id := range task.Dependencies {
			dependency, exists := indices[id]
			if !exists {
				return nil, fmt.Errorf("task %q: unknown dependency %q", task.ID, id)
			}
			dependents[dependency] = append(dependents[dependency], i)
		}
	}

	ordered := make([]Task, 0, len(tasks))
	for len(ordered) < len(tasks) {
		ready := -1
		for i, count := range remaining {
			if count == 0 {
				ready = i
				break
			}
		}
		if ready == -1 {
			// These tasks include both cycle members and tasks blocked by cycles.
			var blocked []string
			for i, count := range remaining {
				if count > 0 {
					blocked = append(blocked, tasks[i].ID)
				}
			}
			return nil, fmt.Errorf("dependency cycle: unresolved tasks %q", blocked)
		}
		ordered = append(ordered, tasks[ready])
		remaining[ready] = -1 // Already emitted; do not select again.
		for _, dependent := range dependents[ready] {
			remaining[dependent]--
		}
	}
	return ordered, nil
}
