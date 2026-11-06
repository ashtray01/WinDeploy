package engine

import (
	"context"
	"errors"
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
	return RunWithOptions(ctx, tasks, adapter, RunOptions{})
}

// RunOptions controls execution. Zero values preserve single-attempt execution.
type RunOptions struct {
	MaxAttempts int
	RetryDelay  time.Duration
}

// RetryableError is an explicit promise by an adapter that retrying is safe.
// Each retry starts with Precheck to detect effects of the previous attempt.
// Ordinary errors are never retried, even if MaxAttempts is greater than one.
type RetryableError interface {
	error
	Retryable() bool
}

// RunWithOptions adds opt-in retries to Run. A task timeout covers every attempt
// and retry delay. Adapters must not mark unsafe operations as retryable.
func RunWithOptions(ctx context.Context, tasks []Task, adapter Adapter, options RunOptions) ([]TaskResult, error) {
	if options.MaxAttempts < 0 || options.RetryDelay < 0 {
		return nil, fmt.Errorf("retry options must not be negative")
	}
	ordered, err := pendingPlan(tasks)
	if err != nil {
		return nil, err
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
				result = runTask(ctx, task, adapter, options)
			}
		}
		result.FinishedAt = time.Now().UTC()
		results = append(results, result)
		statuses[task.ID] = result.Status
	}
	return results, ctx.Err()
}

func runTask(parent context.Context, task Task, adapter Adapter, options RunOptions) TaskResult {
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
	limit := options.MaxAttempts
	if limit == 0 {
		limit = 1
	}
	for {
		if ctx.Err() != nil {
			break
		}
		result.Attempts++
		phase := "precheck"
		err := func() error {
			status, err := adapter.Precheck(ctx, task)
			if err != nil || ctx.Err() != nil {
				return err
			}
			switch status {
			case AlreadyInstalled, NotApplicable, Skipped:
				result.Status = status
				return nil
			case Pending:
			default:
				return fmt.Errorf("invalid precheck outcome")
			}
			phase = "execute"
			reboot, err := adapter.Execute(ctx, task)
			result.RebootRequired = result.RebootRequired || reboot
			if err != nil || ctx.Err() != nil {
				return err
			}
			phase = "verify"
			if err := adapter.Verify(ctx, task); err != nil {
				return err
			}
			result.Status = Success
			return nil
		}()
		if ctx.Err() != nil {
			break
		}
		if err == nil {
			result.Error = ""
			return result
		}
		result.Status, result.Error = Failed, phase+" failed"
		var retry RetryableError
		if result.Attempts >= limit || !errors.As(err, &retry) || !retry.Retryable() {
			return result
		}
		timer := time.NewTimer(options.RetryDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	if parent.Err() != nil {
		result.Status, result.Error = Cancelled, "plan cancelled"
	} else {
		result.Status, result.Error = Failed, "task deadline exceeded"
	}
	return result
}
