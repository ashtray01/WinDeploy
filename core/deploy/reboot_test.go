package deploy

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"windeploy/core/engine"
)

type rebootAdapter struct {
	executed, verified []string
	pending            bool
	verifyErr          error
	rebootTask         string
}

func (a *rebootAdapter) Precheck(context.Context, engine.Task) (engine.TaskStatus, error) {
	return engine.Pending, nil
}
func (a *rebootAdapter) Execute(_ context.Context, task engine.Task) (bool, error) {
	a.executed = append(a.executed, task.ID)
	if task.ID == a.rebootTask {
		a.pending = true
		return true, nil
	}
	return false, nil
}
func (a *rebootAdapter) Verify(_ context.Context, task engine.Task) error {
	a.verified = append(a.verified, task.ID)
	if a.pending && task.ID == a.rebootTask {
		return engine.ErrRebootPending
	}
	return a.verifyErr
}

func TestRebootSessionPersistsAndResumesWithoutReinstall(t *testing.T) {
	ctx := context.Background()
	a := &rebootAdapter{rebootTask: "one"}
	runner, _ := newRunner(t, a)
	first, err := runner.Start(ctx, sampleTasks())
	if err != nil || first.Completed || !first.RebootRequired || first.Summarize().Succeeded != 0 || len(a.executed) != 1 {
		t.Fatalf("false success: %+v err=%v executed=%v", first, err, a.executed)
	}
	data, err := os.ReadFile(first.ReportJSON)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"status": "RebootRequired"`) {
		t.Fatalf("pending evidence absent from report: %s", data)
	}
	// Same boot and RetryFailed consent both leave the file and adapter untouched.
	for _, retry := range []bool{false, true} {
		if _, err := runner.Resume(ctx, first.ID, retry); !errors.Is(err, engine.ErrRebootPending) {
			t.Fatalf("same boot accepted: %v", err)
		}
	}
	if len(a.executed) != 1 {
		t.Fatal("same boot repeated execution")
	}
	stored, err := runner.Store.Load(ctx, first.ID)
	if err != nil || stored.RebootBootIDs["one"] != "boot-one" {
		t.Fatalf("checkpoint evidence: %+v %v", stored, err)
	}
	// A new Runner/adapter models process restart, then a different boot.
	fresh := &rebootAdapter{}
	runner.Adapter = fresh
	runner.ReadBootID = func(context.Context) (string, error) { return "boot-two", nil }
	resumed, err := runner.Resume(ctx, first.ID, false)
	if err != nil || !resumed.Completed || resumed.RebootRequired || resumed.Summarize().Succeeded != 2 {
		t.Fatalf("resume=%+v err=%v", resumed, err)
	}
	if len(fresh.executed) != 1 || fresh.executed[0] != "two" || len(fresh.verified) != 2 || fresh.verified[0] != "one" {
		t.Fatalf("reinstall/verification order: execute=%v verify=%v", fresh.executed, fresh.verified)
	}
}

func TestRebootVerificationFailureDoesNotReleaseDependencies(t *testing.T) {
	ctx := context.Background()
	a := &rebootAdapter{rebootTask: "one"}
	runner, _ := newRunner(t, a)
	first, err := runner.Start(ctx, sampleTasks())
	if err != nil {
		t.Fatal(err)
	}
	fresh := &rebootAdapter{verifyErr: errors.New("unknown state")}
	runner.Adapter = fresh
	runner.ReadBootID = func(context.Context) (string, error) { return "boot-two", nil }
	resumed, err := runner.Resume(ctx, first.ID, false)
	if err != nil || resumed.Completed || resumed.Snapshot.Results[0].Status != engine.RebootRequired || len(fresh.executed) != 0 {
		t.Fatalf("false readiness: %+v err=%v execute=%v", resumed, err, fresh.executed)
	}
	fresh.verifyErr = nil
	resumed, err = runner.Resume(ctx, first.ID, false)
	if err != nil || !resumed.Completed || len(fresh.executed) != 1 {
		t.Fatalf("verify retry failed: %+v %v", resumed, err)
	}
}

func TestOldRebootSessionWithoutBootEvidenceFailsClosed(t *testing.T) {
	ctx := context.Background()
	a := &rebootAdapter{rebootTask: "one"}
	runner, _ := newRunner(t, a)
	session, err := runner.Start(ctx, sampleTasks())
	if err != nil {
		t.Fatal(err)
	}
	session.BootID = ""
	session.RebootBootIDs = nil
	if err := runner.Store.Save(ctx, session); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Resume(ctx, session.ID, true); err == nil {
		t.Fatal("session without evidence accepted")
	}
	if len(a.executed) != 1 {
		t.Fatal("untrusted resume mutated")
	}
}

func TestNewRebootOnResumeRequiresAnotherBoot(t *testing.T) {
	ctx := context.Background()
	runner, _ := newRunner(t, &rebootAdapter{rebootTask: "one"})
	first, err := runner.Start(ctx, sampleTasks())
	if err != nil {
		t.Fatal(err)
	}
	secondAdapter := &rebootAdapter{rebootTask: "two"}
	runner.Adapter = secondAdapter
	runner.ReadBootID = func(context.Context) (string, error) { return "boot-two", nil }
	second, err := runner.Resume(ctx, first.ID, false)
	if err != nil || second.Completed || second.RebootBootIDs["two"] != "boot-two" {
		t.Fatalf("second boundary: %+v %v", second, err)
	}
	if _, err := runner.Resume(ctx, first.ID, true); !errors.Is(err, engine.ErrRebootPending) {
		t.Fatalf("new reboot bypassed: %v", err)
	}
	runner.Adapter = &rebootAdapter{}
	runner.ReadBootID = func(context.Context) (string, error) { return "boot-three", nil }
	third, err := runner.Resume(ctx, first.ID, false)
	if err != nil || !third.Completed || third.RebootRequired {
		t.Fatalf("third boot: %+v %v", third, err)
	}
}

func TestUnavailableBootEvidenceCannotMutate(t *testing.T) {
	ctx := context.Background()
	adapter := &phaseCounter{}
	runner, _ := newRunner(t, adapter)
	runner.ReadBootID = func(context.Context) (string, error) { return "", errors.New("unavailable") }
	if _, err := runner.Start(ctx, sampleTasks()); err == nil || adapter.calls != 0 {
		t.Fatalf("unknown boot mutated: calls=%d err=%v", adapter.calls, err)
	}
	runner.ReadBootID = func(context.Context) (string, error) { return "one", nil }
	session, err := runner.Start(ctx, sampleTasks())
	if err != nil {
		t.Fatal(err)
	}
	priorCalls := adapter.calls
	runner.ReadBootID = func(context.Context) (string, error) { return "", nil }
	if _, err := runner.Resume(ctx, session.ID, true); err == nil || adapter.calls != priorCalls {
		t.Fatalf("unknown resume mutated: calls=%d err=%v", adapter.calls, err)
	}
}
