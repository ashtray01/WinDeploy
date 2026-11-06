package engine

import (
	"context"
	"fmt"
	"time"
)

// Adapter implements task operations. Precheck must check applicability (including
// OS requirements) and existing state, returning Pending to execute or one of
// AlreadyInstalled, NotApplicable, Skipped to finish without execution.
// Execute reports whether a reboot is needed; Verify must confirm actual success.
// Methods must honor ctx, clean up before returning and never mutate Task's nested
// maps/slices. The executor waits for methods to return, even after cancellation;
// it cannot safely interrupt an adapter that ignores context.
type Adapter interface {
	Precheck(context.Context, Task) (TaskStatus, error)
	Execute(context.Context, Task) (rebootRequired bool, err error)
	Verify(context.Context, Task) error
}

// Run executes a Pending-only plan sequentially in OrderTasks order. No operation
// starts until the whole plan is validated. Timeout covers all three phases;
// zero means no additional task deadline (the parent deadline still applies).
// Only Success and AlreadyInstalled satisfy dependencies. Success with a reboot
// flag means Verify confirmed readiness; otherwise Verify must return an error.
//
// Task failures are recorded in results and independent tasks continue. The error
// return is reserved for invalid input or parent cancellation; on cancellation,
// results also include unstarted tasks as Cancelled with zero StartedAt.
// Diagnostics deliberately contain only the failed phase, never raw adapter errors
// which may include credentials. Detailed sanitized diagnostics are future work.
// Input tasks are not updated; resume/retry of non-Pending tasks is not supported.
func Run(ctx context.Context, tasks []Task, adapter Adapter) ([]TaskResult, error) {
	ordered, err := OrderTasks(tasks)
	if err != nil {
		return nil, err
	}
	for _, task := range ordered {
		if task.Status != Pending {
			return nil, fmt.Errorf("task %q must be Pending; resume is not supported", task.ID)
		}
	}
	if len(ordered) > 0 && adapter == nil {
		return nil, fmt.Errorf("task adapter is required")
	}

	results := make([]TaskResult, 0, len(ordered))
	statuses := make(map[string]TaskStatus, len(ordered))
	for _, task := range ordered {
		result := TaskResult{TaskID: task.ID}
		if ctx.Err() != nil {
			result.Status = Cancelled
			result.Error = "plan cancelled"
		} else {
			blocked := false
			for _, id := range task.Dependencies {
				if statuses[id] != Success && statuses[id] != AlreadyInstalled {
					blocked = true
					break
				}
			}
			if blocked {
				result.Status = Skipped
				result.Error = "dependency did not succeed"
			} else {
				result = runTask(ctx, task, adapter)
			}
		}
		result.FinishedAt = time.Now().UTC()
		results = append(results, result)
		statuses[task.ID] = result.Status
	}
	return results, ctx.Err()
}

func runTask(parent context.Context, task Task, adapter Adapter) TaskResult {
	result := TaskResult{TaskID: task.ID, StartedAt: time.Now().UTC()}
	var ctx context.Context
	var cancel context.CancelFunc
	if task.Timeout > 0 {
		ctx, cancel = context.WithTimeout(parent, task.Timeout)
	} else {
		ctx, cancel = context.WithCancel(parent)
	}
	defer cancel()
	task.Status = Running

	// Check context both before and after each call: an adapter may return nil
	// after cancellation, which must not be mistaken for a successful phase.
	check := func(phase string, err error) bool {
		switch {
		case parent.Err() != nil:
			result.Status, result.Error = Cancelled, "plan cancelled"
		case ctx.Err() != nil:
			result.Status, result.Error = Failed, "task deadline exceeded"
		case err != nil:
			result.Status, result.Error = Failed, phase+" failed"
		default:
			return true
		}
		return false
	}
	if !check("precheck", nil) {
		return result
	}
	status, err := adapter.Precheck(ctx, task)
	if !check("precheck", err) {
		return result
	}
	switch status {
	case AlreadyInstalled, NotApplicable, Skipped:
		result.Status = status
		return result
	case Pending:
		// Continue with execution.
	default:
		result.Status, result.Error = Failed, "precheck returned an invalid outcome"
		return result
	}
	if !check("execute", nil) {
		return result
	}
	result.RebootRequired, err = adapter.Execute(ctx, task)
	if !check("execute", err) {
		return result
	}
	if !check("verify", nil) {
		return result
	}
	err = adapter.Verify(ctx, task)
	if check("verify", err) {
		result.Status = Success
	}
	return result
}
