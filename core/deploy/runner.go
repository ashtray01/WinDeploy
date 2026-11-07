package deploy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"windeploy/core/engine"
	"windeploy/core/inventory"
	"windeploy/core/reporting"
)

// Runner executes deployment tasks with checkpoint persistence and optional
// HTML/JSON reports.
type Runner struct {
	Adapter     engine.Adapter
	Store       Store
	ProfileID   string
	ProfileName string
	System      inventory.SystemInfo
	// ReadSystem obtains execution facts on every Start/Resume, never from a saved session.
	// Nil uses the native provider. System is report metadata, not authorization.
	ReadSystem func(context.Context) (inventory.SystemInfo, error)
	ReadBootID func(context.Context) (string, error)
	Options    engine.RunOptions
	ReportDir  string
	// SessionID is optional; empty generates a new ID (tests and callers that
	// need to poll the session file may supply one).
	SessionID string
}

// Start runs the original Pending plan. A session file exists before the first
// operation, so a crash leaves a resumable record.
func (r *Runner) Start(ctx context.Context, tasks []engine.Task) (Session, error) {
	if r.Adapter == nil || r.Store.Dir == "" {
		return Session{}, fmt.Errorf("deployment adapter and session store are required")
	}
	snapshot, err := engine.NewSnapshot(tasks, nil)
	if err != nil {
		return Session{}, err
	}
	adapter, err := r.executionAdapter(ctx, tasks)
	if err != nil {
		return Session{}, err
	}
	bootID, err := r.bootID(ctx)
	if err != nil {
		return Session{}, err
	}
	now := time.Now().UTC()
	sessionID := r.SessionID
	if strings.TrimSpace(sessionID) == "" {
		sessionID = NewSessionID()
	}
	session := Session{
		Version: SessionVersion, ID: sessionID, ProfileID: r.ProfileID, ProfileName: r.ProfileName,
		CreatedAt: now, UpdatedAt: now, StartedAt: now, Tasks: tasks, Snapshot: snapshot, System: r.System, BootID: bootID,
	}
	if err := r.Store.Save(context.Background(), session); err != nil {
		return Session{}, err
	}
	checkpoint := &checkpointStore{store: r.Store, session: session, bootID: bootID}
	options := r.Options
	options.Store = checkpoint
	results, runErr := engine.RunWithOptions(ctx, tasks, adapter, options)
	return r.finish(ctx, checkpoint.session, tasks, results, runErr)
}

// Resume continues a saved session. Failed tasks are retried only with explicit
// RetryFailed consent; completed sessions refuse to run again otherwise.
func (r *Runner) Resume(ctx context.Context, id string, retryFailed bool) (Session, error) {
	if r.Adapter == nil {
		return Session{}, fmt.Errorf("deployment adapter is required")
	}
	session, err := r.Store.Load(ctx, id)
	if err != nil {
		return Session{}, err
	}
	if session.Completed && !retryFailed {
		// Historical sessions can contain an unverified reboot Success.
		if !hasRebootEvidence(session.Snapshot.Results) {
			return session, fmt.Errorf("deployment session is already complete")
		}
	}
	if hasRebootEvidence(session.Snapshot.Results) && session.BootID == "" {
		return session, fmt.Errorf("legacy reboot session lacks trusted boot evidence; automatic resume refused")
	}
	adapter, err := r.executionAdapter(ctx, session.Tasks)
	if err != nil {
		return session, err
	}
	session.System = r.System
	bootID, err := r.bootID(ctx)
	if err != nil {
		return session, err
	}
	verifyReboot := false
	if hasRebootEvidence(session.Snapshot.Results) {
		for _, result := range session.Snapshot.Results {
			if result.RebootRequired {
				origin := session.RebootBootIDs[result.TaskID]
				if origin == "" {
					origin = session.BootID
				}
				if bootID == origin {
					return session, engine.ErrRebootPending
				}
			}
		}
		verifyReboot = true
		for i := range session.Snapshot.Results {
			result := &session.Snapshot.Results[i]
			if result.RebootRequired {
				if result.StartedAt.IsZero() {
					return session, fmt.Errorf("reboot task lacks execution evidence")
				}
				result.Status = engine.RebootRequired
			}
		}
	}
	session.Completed = false
	checkpoint := &checkpointStore{store: r.Store, session: session, bootID: bootID}
	options := engine.ResumeOptions{Execution: r.Options, RetryFailed: retryFailed, VerifyReboot: verifyReboot}
	options.Execution.Store = checkpoint
	results, runErr := engine.Resume(ctx, session.Tasks, adapter, session.Snapshot, options)
	if runErr != nil && results == nil {
		return session, runErr
	}
	return r.finish(ctx, checkpoint.session, session.Tasks, results, runErr)
}

func hasRebootEvidence(results []engine.TaskResult) bool {
	for _, result := range results {
		if result.RebootRequired {
			return true
		}
	}
	return false
}

func (r *Runner) bootID(ctx context.Context) (string, error) {
	read := r.ReadBootID
	if read == nil {
		read = inventory.Native{}.BootID
	}
	id, err := read(ctx)
	if err != nil || strings.TrimSpace(id) == "" {
		return "", fmt.Errorf("deployment boot identity unavailable")
	}
	return id, nil
}

// Validate the entire plan before any adapter phase, including retained resume tasks.
func (r *Runner) executionAdapter(ctx context.Context, tasks []engine.Task) (engine.Adapter, error) {
	read := r.ReadSystem
	if read == nil {
		read = inventory.Native{}.System
	}
	system, err := read(ctx)
	if err != nil {
		return nil, fmt.Errorf("deployment target could not be read: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	guard, err := inventory.GuardAdapter(system, r.Adapter)
	if err != nil {
		return nil, err
	}
	for _, task := range tasks {
		ok, err := inventory.Applicable(task.Requirements, system)
		if err != nil {
			return nil, fmt.Errorf("task %s requirements: %w", task.ID, err)
		}
		if !ok {
			return nil, fmt.Errorf("task %s: %w", task.ID, inventory.ErrNotApplicable)
		}
	}
	r.System = system
	return guard, nil
}

func (r *Runner) finish(ctx context.Context, session Session, tasks []engine.Task, results []engine.TaskResult, runErr error) (Session, error) {
	now := time.Now().UTC()
	session.UpdatedAt = now
	session.FinishedAt = now
	session.RebootRequired = false
	for _, result := range results {
		session.RebootRequired = session.RebootRequired || result.RebootRequired
	}
	session.Cancelled = errors.Is(runErr, context.Canceled) || errors.Is(runErr, context.DeadlineExceeded)
	for _, result := range results {
		if result.Status == engine.Cancelled {
			session.Cancelled = true
		}
	}
	snapshot, err := engine.NewSnapshot(tasks, results)
	if err != nil {
		return session, err
	}
	session.Snapshot = snapshot
	session.Completed = runErr == nil
	for _, result := range results {
		if result.Status == engine.RebootRequired {
			session.Completed = false
		}
	}
	if r.ReportDir != "" {
		report, buildErr := reporting.Build(session.ID, r.ProfileID, r.ProfileName, r.System, tasks, results, session.StartedAt, now, session.RebootRequired, session.Cancelled)
		if buildErr != nil {
			session.ReportError = "отчёт не сформирован"
		} else if jsonPath, htmlPath, saveErr := reporting.Save(r.ReportDir, report); saveErr != nil {
			session.ReportError = "отчёт не сохранён"
		} else {
			session.ReportJSON, session.ReportHTML = jsonPath, htmlPath
		}
	}
	if saveErr := r.Store.Save(context.Background(), session); saveErr != nil {
		return session, fmt.Errorf("deployment session could not be saved")
	}
	if runErr != nil {
		return session, runErr
	}
	return session, nil
}

// checkpointStore writes the full session on every engine checkpoint.
type checkpointStore struct {
	store   Store
	session Session
	bootID  string
}

func (c *checkpointStore) Save(ctx context.Context, snapshot engine.Snapshot) error {
	if c.session.RebootBootIDs == nil {
		c.session.RebootBootIDs = map[string]string{}
	}
	for _, result := range snapshot.Results {
		if result.RebootRequired {
			if c.session.RebootBootIDs[result.TaskID] == "" {
				c.session.RebootBootIDs[result.TaskID] = c.bootID
			}
		} else {
			delete(c.session.RebootBootIDs, result.TaskID)
		}
	}
	c.session.Snapshot = snapshot
	c.session.UpdatedAt = time.Now().UTC()
	return c.store.Save(ctx, c.session)
}
