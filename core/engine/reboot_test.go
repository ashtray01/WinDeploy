package engine

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func TestPendingRebootBlocksDependenciesAndResumesVerifyOnly(t *testing.T) {
	tasks := []Task{planTask("a"), planTask("b", "a")}
	adapter := &fakeAdapter{execute: func(context.Context, Task) (bool, error) { return true, nil }, verify: func(context.Context, Task) error { return ErrRebootPending }}
	results, err := Run(context.Background(), tasks, adapter)
	if err != nil || results[0].Status != RebootRequired || results[1].Status != Skipped {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	snapshot, err := NewSnapshot(tasks, results)
	if err != nil {
		t.Fatal(err)
	}
	adapter = &fakeAdapter{}
	if _, err := Resume(context.Background(), tasks, adapter, snapshot, ResumeOptions{}); err != nil {
		t.Fatal(err)
	}
	if len(adapter.calls) != 0 {
		t.Fatalf("unconfirmed reboot invoked adapter: %v", adapter.calls)
	}
	results, err = Resume(context.Background(), tasks, adapter, snapshot, ResumeOptions{VerifyReboot: true})
	if err != nil || results[0].Status != Success || results[0].RebootRequired || results[0].Attempts != 1 || results[1].Status != Success {
		t.Fatalf("results=%+v err=%v", results, err)
	}
	if !reflect.DeepEqual(adapter.calls, []string{"a:verify", "b:precheck", "b:execute", "b:verify"}) {
		t.Fatalf("repeated execution: %v", adapter.calls)
	}
}

type pendingEvidenceStore struct{ snapshots []Snapshot }

func (s *pendingEvidenceStore) Save(_ context.Context, snapshot Snapshot) error {
	s.snapshots = append(s.snapshots, snapshot)
	return nil
}

func TestRebootVerificationFailureAndCancellationPreserveRecovery(t *testing.T) {
	for _, mode := range []string{"failure", "cancel", "cancel-before"} {
		t.Run(mode, func(t *testing.T) {
			tasks := []Task{planTask("a"), planTask("b", "a")}
			prior := completedResult("a", RebootRequired)
			prior.RebootRequired = true
			snapshot, err := NewSnapshot(tasks, []TaskResult{prior})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if mode == "cancel-before" {
				cancel()
			}
			store := &pendingEvidenceStore{}
			adapter := &fakeAdapter{verify: func(context.Context, Task) error {
				if mode == "cancel" {
					cancel()
				}
				return errors.New("private verification failure")
			}}
			results, _ := Resume(ctx, tasks, adapter, snapshot, ResumeOptions{VerifyReboot: true, Execution: RunOptions{Store: store}})
			if results[0].Status != RebootRequired || !results[0].RebootRequired {
				t.Fatalf("obligation lost: %+v", results)
			}
			for _, snap := range store.snapshots {
				for _, result := range snap.Results {
					if result.TaskID == "a" && result.Status != RebootRequired {
						t.Fatalf("crash would repeat Execute: %+v", result)
					}
				}
			}
			for _, call := range adapter.calls {
				if call != "a:verify" {
					t.Fatalf("mutation/dependency executed: %s", call)
				}
			}
		})
	}
}
