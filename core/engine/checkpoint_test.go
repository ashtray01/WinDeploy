package engine

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryStore struct {
	snapshots []Snapshot
	failAt    int
}

func TestResumeInitialCheckpointPreservesInterruptedEvidence(t *testing.T) {
	tasks := []Task{planTask("a")}
	prior := TaskResult{TaskID: "a", Status: Running, StartedAt: time.Now().UTC(), RebootRequired: true, Attempts: 2}
	snapshot, err := NewSnapshot(tasks, []TaskResult{prior})
	if err != nil {
		t.Fatal(err)
	}
	store := &memoryStore{}
	_, err = Resume(context.Background(), tasks, &fakeAdapter{}, snapshot, ResumeOptions{Execution: RunOptions{Store: store}})
	if err != nil {
		t.Fatal(err)
	}
	first := store.snapshots[0]
	if len(first.Results) != 1 || !first.Results[0].RebootRequired || first.Results[0].Attempts != 2 || first.Results[0].Status != Running {
		t.Fatalf("lost prior evidence: %+v", first)
	}
}

func TestCheckpointFailureCancelsActiveAdapter(t *testing.T) {
	entered := make(chan struct{})
	exited := make(chan struct{})
	store := &blockingFailureStore{entered: entered}
	a := parallelAdapter{execute: func(ctx context.Context, task Task) (bool, error) {
		if task.ID != "a" {
			return false, errors.New("unexpected operation")
		}
		close(entered)
		<-ctx.Done()
		close(exited)
		return false, nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := RunWithOptions(ctx, []Task{planTask("a"), planTask("b")}, a, RunOptions{Store: store, MaxParallel: 2})
	if !errors.Is(err, ErrCheckpoint) {
		t.Fatal(err)
	}
	select {
	case <-exited:
	default:
		t.Fatal("returned before adapter cleanup")
	}
}

type blockingFailureStore struct {
	calls   int
	entered <-chan struct{}
}

func (s *blockingFailureStore) Save(ctx context.Context, _ Snapshot) error {
	s.calls++
	if s.calls == 3 {
		select {
		case <-s.entered:
			return errors.New("storage failure")
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func (s *memoryStore) Save(ctx context.Context, snapshot Snapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := snapshot.Validate(); err != nil {
		return err
	}
	s.snapshots = append(s.snapshots, snapshot)
	if s.failAt == len(s.snapshots) {
		return errors.New("private storage diagnostic")
	}
	return nil
}

func TestCheckpointsBeforeOperationsAndAfterOutcomes(t *testing.T) {
	store := &memoryStore{}
	tasks := []Task{planTask("a"), planTask("b", "a")}
	a := &fakeAdapter{precheck: func(_ context.Context, task Task) (TaskStatus, error) {
		latest := store.snapshots[len(store.snapshots)-1]
		found := false
		for _, r := range latest.Results {
			if r.TaskID == task.ID && r.Status == Running {
				found = true
			}
		}
		if !found {
			t.Error("operation started before durable running checkpoint")
		}
		return Pending, nil
	}}
	results, err := RunWithOptions(context.Background(), tasks, a, RunOptions{Store: store})
	if err != nil || len(results) != 2 {
		t.Fatalf("%+v %v", results, err)
	}
	if len(store.snapshots) != 5 {
		t.Fatalf("checkpoints %d", len(store.snapshots))
	}
	for _, snapshot := range store.snapshots {
		if err := snapshot.ValidateForPlan(tasks); err != nil {
			t.Fatal(err)
		}
	}
	last := store.snapshots[len(store.snapshots)-1]
	if len(last.Results) != 2 || last.Results[0].Status != Success || last.Results[1].Status != Success {
		t.Fatalf("%+v", last)
	}
}

func TestCheckpointFailuresStopNewWork(t *testing.T) {
	for _, failAt := range []int{1, 2, 3} {
		store := &memoryStore{failAt: failAt}
		a := &fakeAdapter{}
		results, err := RunWithOptions(context.Background(), []Task{planTask("a"), planTask("b", "a")}, a, RunOptions{Store: store})
		if !errors.Is(err, ErrCheckpoint) || len(results) != 2 {
			t.Fatalf("%+v %v", results, err)
		}
		want := 0
		if failAt == 3 {
			want = 3
		}
		if len(a.calls) != want {
			t.Fatalf("work continued after failed save: %v", a.calls)
		}
	}
}

func TestCancelledOutcomesPersistWithFreshContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	a := &fakeAdapter{execute: func(context.Context, Task) (bool, error) { cancel(); return false, nil }}
	store := &memoryStore{}
	_, err := RunWithOptions(ctx, []Task{planTask("a"), planTask("b")}, a, RunOptions{Store: store})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	last := store.snapshots[len(store.snapshots)-1]
	if len(last.Results) != 2 || last.Results[0].Status != Cancelled || last.Results[1].Status != Cancelled {
		t.Fatalf("%+v", last)
	}
}
