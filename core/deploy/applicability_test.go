package deploy

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"windeploy/core/engine"
	"windeploy/core/inventory"
)

type phaseCounter struct{ calls int }

func (a *phaseCounter) Precheck(context.Context, engine.Task) (engine.TaskStatus, error) {
	a.calls++
	return engine.Pending, nil
}
func (a *phaseCounter) Execute(context.Context, engine.Task) (bool, error) {
	a.calls++
	return false, nil
}
func (a *phaseCounter) Verify(context.Context, engine.Task) error { a.calls++; return nil }

func TestExecutionApplicabilityStartAndResume(t *testing.T) {
	valid := inventory.SystemInfo{Architecture: "amd64", Windows: inventory.Version{Major: 10, Build: 26100, ProductType: 1}}
	for _, mode := range []string{"start", "resume"} {
		for _, target := range []string{"mismatch", "unknown", "read-error", "cancelled"} {
			t.Run(mode+"/"+target, func(t *testing.T) {
				adapter := &phaseCounter{}
				runner, _ := newRunner(t, adapter)
				tasks := sampleTasks()
				// An unrestricted first task must not run before the later mismatch is found.
				tasks[1].Requirements = engine.OSRequirements{Versions: []string{"windows-11"}}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				reads := 0
				runner.ReadSystem = func(context.Context) (inventory.SystemInfo, error) {
					reads++
					if target == "read-error" {
						return valid, errors.New("unavailable")
					}
					if target == "unknown" {
						return inventory.SystemInfo{}, nil
					}
					system := valid
					system.Windows.Build = 19045
					return system, nil
				}
				var before Session
				if mode == "resume" {
					snapshot, err := engine.NewSnapshot(tasks, nil)
					if err != nil {
						t.Fatal(err)
					}
					before = Session{Version: SessionVersion, ID: NewSessionID(), ProfileID: "office", Tasks: tasks, Snapshot: snapshot, System: valid}
					if err := runner.Store.Save(ctx, before); err != nil {
						t.Fatal(err)
					}
				}
				if target == "cancelled" {
					cancel()
				}
				var err error
				if mode == "start" {
					_, err = runner.Start(ctx, tasks)
				} else {
					_, err = runner.Resume(ctx, before.ID, false)
				}
				if err == nil || adapter.calls != 0 {
					t.Fatalf("err=%v adapter calls=%d", err, adapter.calls)
				}
				if target != "cancelled" && reads != 1 {
					t.Fatalf("fresh reads=%d", reads)
				}
				if mode == "resume" {
					after, loadErr := runner.Store.Load(context.Background(), before.ID)
					if loadErr != nil || !reflect.DeepEqual(after.Snapshot, before.Snapshot) {
						t.Fatalf("rejected resume changed checkpoint: %v", loadErr)
					}
				}
			})
		}
	}
}

func TestResumeReadsFreshTargetAndReportsIt(t *testing.T) {
	adapter := &phaseCounter{}
	runner, _ := newRunner(t, adapter)
	session, err := runner.Start(context.Background(), sampleTasks())
	if err != nil {
		t.Fatal(err)
	}
	reads := 0
	runner.ReadSystem = func(context.Context) (inventory.SystemInfo, error) {
		reads++
		return inventory.SystemInfo{Hostname: "fresh", Architecture: "amd64", Windows: inventory.Version{Major: 10, Build: 19045, ProductType: 1}}, nil
	}
	resumed, err := runner.Resume(context.Background(), session.ID, true)
	if err != nil || reads != 1 || resumed.System.Hostname != "fresh" {
		t.Fatalf("fresh resume: %+v reads=%d err=%v", resumed.System, reads, err)
	}
}
