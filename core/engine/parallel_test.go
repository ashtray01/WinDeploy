package engine

import (
	"context"
	"errors"
	"testing"
	"time"
)

type parallelAdapter struct {
	execute func(context.Context, Task) (bool, error)
}

func (a parallelAdapter) Precheck(context.Context, Task) (TaskStatus, error) { return Pending, nil }
func (a parallelAdapter) Execute(ctx context.Context, t Task) (bool, error)  { return a.execute(ctx, t) }
func (a parallelAdapter) Verify(context.Context, Task) error                 { return nil }

func receiveTask(t *testing.T, ch <-chan string) string {
	t.Helper()
	select {
	case id := <-ch:
		return id
	case <-time.After(5 * time.Second):
		t.Fatal("task did not start")
		return ""
	}
}

func TestBoundedParallelism(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entered := make(chan string, 3)
	release := make(chan struct{}, 3)
	a := parallelAdapter{execute: func(ctx context.Context, task Task) (bool, error) {
		entered <- task.ID
		select {
		case <-release:
			return false, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}}
	type outcome struct {
		results []TaskResult
		err     error
	}
	done := make(chan outcome, 1)
	go func() {
		r, e := RunWithOptions(ctx, []Task{planTask("a"), planTask("b"), planTask("c")}, a, RunOptions{MaxParallel: 2})
		done <- outcome{r, e}
	}()
	first, second := receiveTask(t, entered), receiveTask(t, entered)
	if first == second || first == "c" || second == "c" {
		t.Fatalf("first tasks: %s %s", first, second)
	}
	select {
	case id := <-entered:
		t.Fatalf("limit exceeded: %s", id)
	default:
	}
	release <- struct{}{}
	if id := receiveTask(t, entered); id != "c" {
		t.Fatalf("third task %s", id)
	}
	release <- struct{}{}
	release <- struct{}{}
	out := <-done
	if out.err != nil || len(out.results) != 3 {
		t.Fatalf("%+v %v", out.results, out.err)
	}
	for i, id := range []string{"a", "b", "c"} {
		if out.results[i].TaskID != id || out.results[i].Status != Success {
			t.Fatalf("unstable results: %+v", out.results)
		}
	}
}

func TestExclusiveResourcesAndDependencies(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	entered := make(chan string, 4)
	releases := map[string]chan struct{}{"a": make(chan struct{}), "b": make(chan struct{}), "c": make(chan struct{}), "d": make(chan struct{})}
	a := parallelAdapter{execute: func(ctx context.Context, task Task) (bool, error) {
		entered <- task.ID
		select {
		case <-releases[task.ID]:
			return false, nil
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}}
	tasks := []Task{planTask("a"), planTask("b"), planTask("c"), planTask("d", "a", "b")}
	tasks[0].ResourceKey = WindowsInstallerResource
	tasks[1].ResourceKey = WindowsInstallerResource
	done := make(chan []TaskResult, 1)
	go func() { r, _ := RunWithOptions(ctx, tasks, a, RunOptions{MaxParallel: 4}); done <- r }()
	x, y := receiveTask(t, entered), receiveTask(t, entered)
	if !((x == "a" && y == "c") || (x == "c" && y == "a")) {
		t.Fatalf("resource/dependency violation: %s %s", x, y)
	}
	close(releases["a"])
	if id := receiveTask(t, entered); id != "b" {
		t.Fatalf("expected b, got %s", id)
	}
	select {
	case id := <-entered:
		t.Fatalf("dependency started early: %s", id)
	default:
	}
	close(releases["b"])
	if id := receiveTask(t, entered); id != "d" {
		t.Fatalf("expected d, got %s", id)
	}
	close(releases["c"])
	close(releases["d"])
	for _, r := range <-done {
		if r.Status != Success {
			t.Fatalf("%+v", r)
		}
	}
}

func TestParallelCancellationDrainsActiveTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	entered := make(chan string, 2)
	exited := make(chan string, 2)
	a := parallelAdapter{execute: func(ctx context.Context, task Task) (bool, error) {
		entered <- task.ID
		<-ctx.Done()
		exited <- task.ID
		return false, nil
	}}
	done := make(chan error, 1)
	go func() {
		r, e := RunWithOptions(ctx, []Task{planTask("a"), planTask("b"), planTask("c")}, a, RunOptions{MaxParallel: 2})
		if len(r) != 3 {
			done <- errors.New("missing results")
			return
		}
		for _, v := range r {
			if v.Status != Cancelled {
				done <- errors.New("wrong status")
				return
			}
		}
		done <- e
	}()
	receiveTask(t, entered)
	receiveTask(t, entered)
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel stuck")
	}
	if len(exited) != 2 {
		t.Fatal("returned before active adapters exited")
	}
	select {
	case id := <-entered:
		t.Fatalf("started after cancellation: %s", id)
	default:
	}
}

func TestInvalidParallelOptionsAndResource(t *testing.T) {
	if _, e := RunWithOptions(context.Background(), nil, nil, RunOptions{MaxParallel: -1}); e == nil {
		t.Fatal("negative parallelism accepted")
	}
	task := planTask("a")
	task.ResourceKey = " "
	if _, e := OrderTasks([]Task{task}); e == nil {
		t.Fatal("whitespace resource accepted")
	}
}
