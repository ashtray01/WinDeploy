package engine

import (
	"context"
	"fmt"
)

// ResumeOptions requires explicit consent to repeat terminal failed tasks.
type ResumeOptions struct {
	Execution   RunOptions
	RetryFailed bool
	// VerifyReboot permits Verify-only for pending reboot tasks. The caller must
	// first prove a different boot identity; zero keeps the barrier closed.
	VerifyReboot bool
}

// ValidateForPlan validates binding and started dependencies, not malicious
// modification or live inventory. A snapshot belongs in trusted local storage.
func (s Snapshot) ValidateForPlan(tasks []Task) error {
	if err := s.Validate(); err != nil {
		return err
	}
	expected, err := NewSnapshot(tasks, nil)
	if err != nil {
		return err
	}
	if expected.PlanHash != s.PlanHash {
		return fmt.Errorf("snapshot does not match the plan")
	}
	byID := make(map[string]Task, len(tasks))
	for _, task := range tasks {
		byID[task.ID] = task
	}
	results := make(map[string]TaskResult, len(s.Results))
	for _, result := range s.Results {
		if _, ok := byID[result.TaskID]; !ok {
			return fmt.Errorf("snapshot contains unknown task")
		}
		results[result.TaskID] = result
	}
	for _, result := range s.Results {
		if result.StartedAt.IsZero() {
			continue
		}
		for _, id := range byID[result.TaskID].Dependencies {
			status := results[id].Status
			if status != Success && status != AlreadyInstalled {
				return fmt.Errorf("snapshot task has unsatisfied dependencies")
			}
		}
	}
	return nil
}

func retainResult(result TaskResult, retryFailed bool) bool {
	switch result.Status {
	case Success, AlreadyInstalled, NotApplicable, RebootRequired:
		return true
	case Skipped:
		return result.Error != "dependency did not succeed"
	case Failed:
		return !retryFailed
	default:
		return false
	}
}

// Resume keeps trusted completed outcomes without a live inventory recheck.
// Failed tasks stay failed unless RetryFailed is true. Running/Cancelled tasks
// and dependency-blocked skips return through Precheck, which must safely detect
// partial/installed effects before Execute. Supply the original Pending plan.
// Pending reboot results use Verify-only when VerifyReboot is authorized.
// Successful verification clears that reboot obligation; attempts are unchanged.
func Resume(ctx context.Context, tasks []Task, adapter Adapter, snapshot Snapshot, options ResumeOptions) ([]TaskResult, error) {
	if err := snapshot.ValidateForPlan(tasks); err != nil {
		return nil, err
	}
	previous := make(map[string]TaskResult, len(snapshot.Results))
	for _, result := range snapshot.Results {
		previous[result.TaskID] = result
	}
	return runPlan(ctx, tasks, adapter, options.Execution, previous, options.RetryFailed, options.VerifyReboot)
}
