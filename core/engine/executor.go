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
// An adapter may implement WarningReporter to report effects that were applied but
// not confirmed; those warnings are persisted with the task result.
// Methods must honor ctx, clean up before returning and never mutate Task's nested
// maps/slices. The executor waits for methods to return, even after cancellation;
// it cannot safely interrupt an adapter that ignores context.
type Adapter interface {
	Precheck(context.Context, Task) (TaskStatus, error)
	Execute(context.Context, Task) (rebootRequired bool, err error)
	Verify(context.Context, Task) error
}

// WarningReporter is an optional Adapter capability. Warnings must be short,
// translated user notices without credentials or raw command output. The executor
// copies them after Execute and Verify, so an adapter never mutates results.
type WarningReporter interface {
	TaskWarnings(Task) []string
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
// Input tasks are not updated; use Resume to continue a saved session.
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
	// Store receives a checkpoint before any operation and after each outcome.
	// Save failure stops new tasks, cancels active operations and returns
	// ErrCheckpoint. The store must honor context and have one session writer.
	Store SnapshotStore
}

type SnapshotStore interface {
	Save(context.Context, Snapshot) error
}

// ErrCheckpoint deliberately excludes raw storage diagnostics.
var ErrCheckpoint = errors.New("could not save deployment checkpoint")

// ErrRebootPending means execution finished but verification must wait for a
// confirmed reboot. It is an outcome, not permission to repeat Execute.
var ErrRebootPending = errors.New("verification awaits reboot")

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
	return runPlan(ctx, tasks, adapter, options, nil, false, false)
}

func runPlan(ctx context.Context, tasks []Task, adapter Adapter, options RunOptions, previous map[string]TaskResult, retryFailed, verifyReboot bool) ([]TaskResult, error) {
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
	runCtx, stop := context.WithCancel(ctx)
	defer stop()
	var checkpointErr error
	var template Snapshot
	if options.Store != nil {
		template, err = NewSnapshot(ordered, nil)
		if err != nil {
			return nil, err
		}
	}

	results := make([]TaskResult, len(ordered))
	save := func() {
		if options.Store == nil || checkpointErr != nil {
			return
		}
		snapshot := template
		snapshot.Results = make([]TaskResult, 0, len(results))
		for _, result := range results {
			if result.TaskID != "" {
				snapshot.Results = append(snapshot.Results, result)
			}
		}
		// Cancellation must still persist outcomes. Use Go 1.20-compatible
		// detached, bounded context rather than the cancelled operation context.
		saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := options.Store.Save(saveCtx, snapshot); err != nil {
			checkpointErr = ErrCheckpoint
			stop()
		}
	}
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
		if result.StartedAt.IsZero() {
			result.Attempts = previous[ordered[i].ID].Attempts
			result.RebootRequired = previous[ordered[i].ID].RebootRequired
		}
		result.FinishedAt = time.Now().UTC()
		results[i] = result
		statuses[ordered[i].ID] = result.Status
		started[i] = true
		completed++
		save()
	}
	for i, task := range ordered {
		if prior, ok := previous[task.ID]; ok {
			// Keep interrupted evidence in the initial checkpoint too. A crash
			// between Save and the next attempt must not erase prior reboot flags.
			results[i] = prior
			if retainResult(prior, retryFailed) && !(prior.Status == RebootRequired && verifyReboot) {
				statuses[task.ID], started[i] = prior.Status, true
				completed++
			}
		}
	}
	save()
	for completed < len(ordered) {
		for i, task := range ordered {
			if started[i] {
				continue
			}
			if runCtx.Err() != nil {
				if prior, ok := previous[task.ID]; ok && prior.Status == RebootRequired {
					record(i, prior)
					continue
				}
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
			results[i] = TaskResult{TaskID: task.ID, Status: Running, StartedAt: time.Now().UTC(), Attempts: previous[task.ID].Attempts, RebootRequired: previous[task.ID].RebootRequired}
			if prior, ok := previous[task.ID]; ok && prior.Status == RebootRequired {
				// Preserve Verify-only recovery even if the process dies during Verify.
				results[i] = prior
			}
			save()
			if checkpointErr != nil {
				if prior, ok := previous[task.ID]; ok && prior.Status == RebootRequired {
					record(i, prior)
					continue
				}
				record(i, TaskResult{TaskID: task.ID, Status: Cancelled, Error: "plan cancelled"})
				continue
			}
			if task.ResourceKey != "" {
				resources[task.ResourceKey] = true
			}
			active++
			go func(index int, task Task) {
				if prior, ok := previous[task.ID]; ok && prior.Status == RebootRequired {
					finished <- completion{index, verifyAfterReboot(runCtx, task, adapter, prior)}
					return
				}
				result := runTask(runCtx, task, adapter, options)
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
	if checkpointErr != nil {
		return results, checkpointErr
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
			collectWarnings(adapter, task, &result)
			if err != nil || ctx.Err() != nil {
				return err
			}
			phase = "verify"
			if err := adapter.Verify(ctx, task); err != nil {
				return err
			}
			collectWarnings(adapter, task, &result)
			result.Status = Success
			return nil
		}()
		if phase == "verify" && errors.Is(err, ErrRebootPending) {
			result.Status, result.RebootRequired, result.Error = RebootRequired, true, ""
			return result
		}
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

// A persisted reboot obligation never returns through Precheck or Execute.
// Failed/cancelled verification keeps the obligation for a later safe retry.
func verifyAfterReboot(parent context.Context, task Task, adapter Adapter, prior TaskResult) TaskResult {
	ctx, cancel := context.WithCancel(parent)
	if task.Timeout > 0 {
		cancel()
		ctx, cancel = context.WithTimeout(parent, task.Timeout)
	}
	defer cancel()
	result := prior
	result.StartedAt = time.Now().UTC()
	task.Status = Running
	if ctx.Err() != nil {
		result.Error = "plan cancelled"
		if parent.Err() == nil {
			result.Error = "task deadline exceeded"
		}
		return result
	}
	err := adapter.Verify(ctx, task)
	collectWarnings(adapter, task, &result)
	if ctx.Err() != nil {
		result.Error = "plan cancelled"
		if parent.Err() == nil {
			result.Error = "task deadline exceeded"
		}
		return result
	}
	if err != nil {
		result.Error = "verify failed"
		return result
	}
	result.Status, result.RebootRequired, result.Error = Success, false, ""
	return result
}

// collectWarnings copies the adapter's current notices into the result. A
// WarningReporter that panics is ignored: warnings must never break execution.
func collectWarnings(adapter Adapter, task Task, result *TaskResult) {
	reporter, ok := adapter.(WarningReporter)
	if !ok {
		return
	}
	var warnings []string
	func() {
		defer func() { _ = recover() }()
		warnings = reporter.TaskWarnings(task)
	}()
	result.Warnings = SanitizeWarnings(append(append([]string{}, result.Warnings...), warnings...))
}
