package state

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"windeploy/core/engine"
)

type integrationAdapter struct {
	cancel    context.CancelFunc
	installed map[string]bool
	installs  map[string]int
}

func (a *integrationAdapter) Precheck(_ context.Context, task engine.Task) (engine.TaskStatus, error) {
	if a.installed[task.ID] {
		return engine.AlreadyInstalled, nil
	}
	return engine.Pending, nil
}
func (a *integrationAdapter) Execute(_ context.Context, task engine.Task) (bool, error) {
	a.installed[task.ID] = true
	a.installs[task.ID]++
	if task.ID == "b" && a.cancel != nil {
		a.cancel()
	}
	return false, nil
}
func (a *integrationAdapter) Verify(context.Context, engine.Task) error { return nil }

func TestFileCheckpointRestartDoesNotDuplicateInstallation(t *testing.T) {
	tasks := []engine.Task{
		{ID: "a", Name: "A", Category: "test", Status: engine.Pending},
		{ID: "b", Name: "B", Category: "test", Status: engine.Pending, Dependencies: []string{"a"}},
		{ID: "c", Name: "C", Category: "test", Status: engine.Pending, Dependencies: []string{"b"}},
	}
	path := filepath.Join(t.TempDir(), "session.json")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a := &integrationAdapter{cancel: cancel, installed: map[string]bool{}, installs: map[string]int{}}
	_, err := engine.RunWithOptions(ctx, tasks, a, engine.RunOptions{Store: &FileStore{Path: path}})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// A new store instance represents a new process reading the saved state.
	store := &FileStore{Path: path}
	snapshot, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	a.cancel = nil
	results, err := engine.Resume(context.Background(), tasks, a, snapshot, engine.ResumeOptions{Execution: engine.RunOptions{Store: store}})
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "b", "c"} {
		if a.installs[id] != 1 {
			t.Fatalf("%s installed %d times", id, a.installs[id])
		}
	}
	if results[0].Status != engine.Success || results[1].Status != engine.AlreadyInstalled || results[2].Status != engine.Success {
		t.Fatalf("%+v", results)
	}
	final, err := store.Load(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err = final.ValidateForPlan(tasks); err != nil {
		t.Fatal(err)
	}
}
