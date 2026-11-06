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
	// MaxParallel defaults to one. For values above one Adapter must be safe
	// for concurrent calls on different tasks. Equal ResourceKey values exclude
	// each other even when retries are waiting.
	MaxParallel int
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
	return runPlan(ctx, tasks, adapter, options, nil, false)
}

func runPlan(ctx context.Context, tasks []Task, adapter Adapter, options RunOptions, previous map[string]TaskResult, retryFailed bool) ([]TaskResult, error) {
	if options.MaxAttempts < 0 || options.RetryDelay < 0 || options.MaxParallel < 0 {
		return nil, fmt.Errorf("execution options must not be negative")
	}
	ordered, err := pendingPlan(tasks)
	if err != nil {
		return nil, err
	}
	if len(ordered) > 0 && adapter == nil {
		return nil, fmt.Errorf("task adapter is required")
	}

	results := make([]TaskResult, len(ordered))
	statuses := make(map[string]TaskStatus, len(ordered))
	started := make([]bool, len(ordered))
	resources := make(map[string]bool)
	type completion struct {
		index  int
		result TaskResult
	}
	finished := make(chan completion, len(ordered))
	limit := options.MaxParallel
	if limit == 0 {
		limit = 1
	}
	active, completed := 0, 0
	record := func(i int, result TaskResult) {
		result.FinishedAt = time.Now().UTC()
		results[i] = result
		statuses[ordered[i].ID] = result.Status
		started[i] = true
		completed++
	}
	for i, task := range ordered {
		if prior, ok := previous[task.ID]; ok && retainResult(prior, retryFailed) {
			results[i], statuses[task.ID], started[i] = prior, prior.Status, true
			completed++
		}
	}
	for completed < len(ordered) {
		for i, task := range ordered {
			if started[i] {
				continue
			}
			if ctx.Err() != nil {
				record(i, TaskResult{TaskID: task.ID, Status: Cancelled, Error: "plan cancelled"})
				continue
			}
			blocked, waiting := false, false
			for _, id := range task.Dependencies {
				switch statuses[id] {
				case "", Pending, Running:
					waiting = true
				case Success, AlreadyInstalled:
				default:
					blocked = true
				}
			}
			if blocked {
				record(i, TaskResult{TaskID: task.ID, Status: Skipped, Error: "dependency did not succeed"})
				continue
			}
			if waiting || active >= limit || (task.ResourceKey != "" && resources[task.ResourceKey]) {
				continue
			}
			started[i] = true
			statuses[task.ID] = Running
			if task.ResourceKey != "" {
				resources[task.ResourceKey] = true
			}
			active++
			go func(index int, task Task) {
				result := runTask(ctx, task, adapter, options)
				if prior, ok := previous[task.ID]; ok {
					result.Attempts += prior.Attempts
					result.RebootRequired = result.RebootRequired || prior.RebootRequired
				}
				finished <- completion{index, result}
			}(i, task)
		}
		if active > 0 {
			done := <-finished // Always drain running operations, including after cancellation.
			active--
			delete(resources, ordered[done.index].ResourceKey)
			record(done.index, done.result)
		}
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
