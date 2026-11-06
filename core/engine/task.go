// Package engine defines deployment tasks independently of platform operations.
package engine

import (
	"fmt"
	"strings"
	"time"
)

// TaskStatus is the recorded state of a task. The zero value is invalid;
// newly planned tasks must explicitly receive Pending.
type TaskStatus string

const (
	Pending          TaskStatus = "Pending"
	Running          TaskStatus = "Running"
	Success          TaskStatus = "Success"
	Failed           TaskStatus = "Failed"
	Skipped          TaskStatus = "Skipped"
	AlreadyInstalled TaskStatus = "AlreadyInstalled"
	NotApplicable    TaskStatus = "NotApplicable"
	Cancelled        TaskStatus = "Cancelled"
	RebootRequired   TaskStatus = "RebootRequired"
)

// Valid reports whether s is a recognized status; it does not check transitions.
func (s TaskStatus) Valid() bool {
	switch s {
	case Pending, Running, Success, Failed, Skipped, AlreadyInstalled,
		NotApplicable, Cancelled, RebootRequired:
		return true
	default:
		return false
	}
}

// OSRequirements describes allowed Windows versions and Go architecture names
// (386, amd64). Empty lists mean no restriction. Evaluation belongs to the
// future inventory/engine integration, not to structural task validation.
type OSRequirements struct {
	Versions      []string `json:"versions,omitempty"`
	Architectures []string `json:"architectures,omitempty"`
}

// Task is a serializable description, not an executable operation. Parameters
// must contain non-secret configuration only; credentials must stay outside it.
// A zero Timeout means the executor's default. Timeout is encoded as nanoseconds.
type Task struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Category     string            `json:"category"`
	Parameters   map[string]string `json:"parameters,omitempty"`
	Requirements OSRequirements    `json:"requirements"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Timeout      time.Duration     `json:"timeout"`
	Status       TaskStatus        `json:"status"`
}

// Validate checks a single task's structure. OrderTasks checks dependency
// references and cycles. Neither checks OS compatibility or status transitions.
func (t Task) Validate() error {
	if strings.TrimSpace(t.ID) == "" {
		return fmt.Errorf("task ID is required")
	}
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("task name is required")
	}
	if strings.TrimSpace(t.Category) == "" {
		return fmt.Errorf("task category is required")
	}
	if t.Timeout < 0 {
		return fmt.Errorf("task timeout must not be negative")
	}
	if !t.Status.Valid() {
		return fmt.Errorf("task status is invalid")
	}
	seen := make(map[string]bool, len(t.Dependencies))
	for _, id := range t.Dependencies {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("dependency ID is required")
		}
		if id == t.ID {
			return fmt.Errorf("task must not depend on itself")
		}
		if seen[id] {
			return fmt.Errorf("duplicate dependency ID")
		}
		seen[id] = true
	}
	return nil
}

// TaskResult records an execution outcome. Error is a sanitized diagnostic,
// not a raw command output; callers must remove credentials before storing it.
// RebootRequired can accompany a successful result without losing its status.
// This model does not yet implement persistence, retries or execution.
type TaskResult struct {
	TaskID         string     `json:"task_id"`
	Status         TaskStatus `json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     time.Time  `json:"finished_at"`
	Error          string     `json:"error,omitempty"`
	RebootRequired bool       `json:"reboot_required"`
}
