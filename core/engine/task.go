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
// (386, amd64). Empty lists mean no restriction within the supported matrix.
// Evaluation is provided by inventory.Applicable/GuardAdapter; Task.Validate
// only checks structural fields and does not read the target system.
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
	// ResourceKey serializes tasks using the same nonempty resource, across
	// precheck, attempts, retry waits and verification. MSI adapters must set
	// WindowsInstallerResource for every Windows Installer operation.
	ResourceKey string `json:"resource_key,omitempty"`
}

const WindowsInstallerResource = "windows-installer"

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
	if t.ResourceKey != "" && strings.TrimSpace(t.ResourceKey) == "" {
		return fmt.Errorf("resource key must not be whitespace")
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
// A RebootRequired status is unverified and never satisfies dependencies.
// A reboot flag on Success is allowed only if Verify confirmed readiness.
type TaskResult struct {
	TaskID         string     `json:"task_id"`
	Status         TaskStatus `json:"status"`
	StartedAt      time.Time  `json:"started_at"`
	FinishedAt     time.Time  `json:"finished_at"`
	Error          string     `json:"error,omitempty"`
	RebootRequired bool       `json:"reboot_required"`
	Attempts       int        `json:"attempts,omitempty"`
	// Warnings are short, already-translated user notices about effects that
	// were not confirmed. They never contain raw diagnostics or credentials.
	Warnings []string `json:"warnings,omitempty"`
}

// MaxWarnings bounds warnings persisted per task.
const MaxWarnings = 8

// WarningTextLimit bounds one warning's length in runes.
const WarningTextLimit = 300

func validateWarnings(warnings []string) error {
	if len(warnings) > MaxWarnings {
		return fmt.Errorf("too many task warnings")
	}
	for _, warning := range warnings {
		if strings.TrimSpace(warning) == "" || len([]rune(warning)) > WarningTextLimit || strings.ContainsAny(warning, "\x00\r\n") {
			return fmt.Errorf("invalid task warning")
		}
	}
	return nil
}

// SanitizeWarnings trims, de-duplicates and bounds warnings for persistence.
func SanitizeWarnings(warnings []string) []string {
	clean := make([]string, 0, len(warnings))
	seen := map[string]bool{}
	for _, warning := range warnings {
		text := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(warning, "\r", " "), "\n", " "))
		if text == "" || seen[text] {
			continue
		}
		if runes := []rune(text); len(runes) > WarningTextLimit {
			text = string(runes[:WarningTextLimit])
		}
		seen[text] = true
		clean = append(clean, text)
		if len(clean) == MaxWarnings {
			break
		}
	}
	return clean
}
