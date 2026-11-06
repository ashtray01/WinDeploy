package engine

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

type fakeAdapter struct {
	calls    []string
	precheck func(context.Context, Task) (TaskStatus, error)
	execute  func(context.Context, Task) (bool, error)
	verify   func(context.Context, Task) error
}

func (a *fakeAdapter) Precheck(ctx context.Context, task Task) (TaskStatus, error) {
	a.calls = append(a.calls, task.ID+":precheck")
	if a.precheck != nil {
		return a.precheck(ctx, task)
	}
	return Pending, nil
}

func (a *fakeAdapter) Execute(ctx context.Context, task Task) (bool, error) {
	a.calls = append(a.calls, task.ID+":execute")
	if a.execute != nil {
		return a.execute(ctx, task)
	}
	return false, nil
}

func (a *fakeAdapter) Verify(ctx context.Context, task Task) error {
	a.calls = append(a.calls, task.ID+":verify")
	if a.verify != nil {
		return a.verify(ctx, task)
	}
	return nil
}

func TestRunSuccess(t *testing.T) {
	tasks := []Task{planTask("b", "a"), planTask("a")}
	a := &fakeAdapter{execute: func(ctx context.Context, task Task) (bool, error) {
		if task.Status != Running {
			t.Errorf("adapter sees status %s", task.Status)
		}
		return task.ID == "a", nil
	}}
	results, err := Run(context.Background(), tasks, a)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a:precheck", "a:execute", "a:verify", "b:precheck", "b:execute", "b:verify"}
	if !reflect.DeepEqual(a.calls, want) {
		t.Fatalf("calls = %v", a.calls)
	}
	if len(results) != 2 || results[0].TaskID != "a" || results[1].TaskID != "b" || !results[0].RebootRequired {
		t.Fatalf("unexpected results: %+v", results)
	}
	for _, result := range results {
		if result.Status != Success || result.Error != "" || result.StartedAt.IsZero() || result.FinishedAt.Before(result.StartedAt) {
			t.Errorf("invalid success result: %+v", result)
		}
	}
	if !reflect.DeepEqual(tasks, []Task{planTask("b", "a"), planTask("a")}) {
		t.Fatal("input mutated")
	}
}

func TestRunRejectsInputBeforeAnyOperation(t *testing.T) {
	done := planTask("done")
	done.Status = Success
	for _, tasks := range [][]Task{
		{planTask("first"), planTask("a", "missing")},
		{planTask("first"), planTask("a", "b"), planTask("b", "a")},
		{planTask("first"), done},
	} {
		a := &fakeAdapter{}
		results, err := Run(context.Background(), tasks, a)
		if err == nil || results != nil || len(a.calls) != 0 {
			t.Fatalf("invalid input executed: %v, %+v, %v", err, results, a.calls)
		}
	}
	if _, err := Run(context.Background(), []Task{planTask("a")}, nil); err == nil {
		t.Fatal("nil adapter accepted")
	}
	if results, err := Run(context.Background(), nil, nil); err != nil || len(results) != 0 {
		t.Fatalf("empty plan: %v, %v", results, err)
	}
}

func TestRunPhaseFailures(t *testing.T) {
	secretErr := errors.New("password=must-never-appear")
	for _, phase := range []string{"precheck", "execute", "verify"} {
		t.Run(phase, func(t *testing.T) {
			a := &fakeAdapter{}
			a.precheck = func(_ context.Context, task Task) (TaskStatus, error) {
				if task.ID == "a" && phase == "precheck" {
					return Pending, secretErr
				}
				return Pending, nil
			}
			a.execute = func(_ context.Context, task Task) (bool, error) {
				if task.ID == "a" && phase == "execute" {
					return true, secretErr
				}
				return false, nil
			}
			a.verify = func(_ context.Context, task Task) error {
				if task.ID == "a" && phase == "verify" {
					return secretErr
				}
				return nil
			}
			results, err := Run(context.Background(), []Task{planTask("a"), planTask("b", "a"), planTask("c", "b"), planTask("independent")}, a)
			if err != nil {
				t.Fatal(err)
			}
			if results[0].Status != Failed || results[0].Error != phase+" failed" || results[1].Status != Skipped || results[2].Status != Skipped || results[3].Status != Success {
				t.Fatalf("unexpected results: %+v", results)
			}
			for _, call := range a.calls {
				if strings.HasPrefix(call, "b:") || strings.HasPrefix(call, "c:") {
					t.Fatalf("blocked task ran: %s", call)
				}
			}
			wantCalls := []string{"a:precheck"}
			if phase != "precheck" {
				wantCalls = append(wantCalls, "a:execute")
			}
			if phase == "verify" {
				wantCalls = append(wantCalls, "a:verify")
			}
			wantCalls = append(wantCalls, "independent:precheck", "independent:execute", "independent:verify")
			if !reflect.DeepEqual(a.calls, wantCalls) {
				t.Fatalf("calls = %v, want %v", a.calls, wantCalls)
			}
		})
	}
}

func TestRunPrecheckOutcomes(t *testing.T) {
	for _, status := range []TaskStatus{AlreadyInstalled, NotApplicable, Skipped, Success, ""} {
		t.Run(string(status), func(t *testing.T) {
			a := &fakeAdapter{precheck: func(_ context.Context, task Task) (TaskStatus, error) {
				if task.ID == "a" {
					return status, nil
				}
				return Pending, nil
			}}
			results, err := Run(context.Background(), []Task{planTask("a"), planTask("b", "a")}, a)
			if err != nil {
				t.Fatal(err)
			}
			want, dependent := status, Skipped
			if status == AlreadyInstalled {
				dependent = Success
			}
			if status == Success || status == "" {
				want = Failed
			}
			if results[0].Status != want || results[1].Status != dependent {
				t.Fatalf("results: %+v", results)
			}
			for _, call := range a.calls {
				if call == "a:execute" || call == "a:verify" {
					t.Fatalf("precheck did not short circuit: %v", a.calls)
				}
			}
		})
	}
}

func TestRunCancellationBeforeStart(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	a := &fakeAdapter{}
	results, err := Run(ctx, []Task{planTask("a"), planTask("b", "a")}, a)
	if !errors.Is(err, context.Canceled) || len(a.calls) != 0 {
		t.Fatalf("calls %v, error %v", a.calls, err)
	}
	for _, result := range results {
		if result.Status != Cancelled || !result.StartedAt.IsZero() || result.FinishedAt.IsZero() {
			t.Fatalf("result: %+v", result)
		}
	}
}

func TestRunCancellationDuringPhases(t *testing.T) {
	for _, phase := range []string{"precheck", "execute", "verify"} {
		t.Run(phase, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			entered := make(chan struct{})
			block := func(ctx context.Context) {
				close(entered)
				<-ctx.Done()
			}
			a := &fakeAdapter{}
			switch phase {
			case "precheck":
				a.precheck = func(ctx context.Context, _ Task) (TaskStatus, error) { block(ctx); return Pending, nil }
			case "execute":
				a.execute = func(ctx context.Context, _ Task) (bool, error) { block(ctx); return false, nil }
			case "verify":
				a.verify = func(ctx context.Context, _ Task) error { block(ctx); return nil }
			}
			type outcome struct {
				results []TaskResult
				err     error
			}
			finished := make(chan outcome, 1)
			go func() {
				results, err := Run(ctx, []Task{planTask("a"), planTask("b")}, a)
				finished <- outcome{results, err}
			}()
			select {
			case <-entered:
				cancel()
			case <-time.After(5 * time.Second):
				t.Fatal("phase never entered")
			}
			select {
			case out := <-finished:
				if !errors.Is(out.err, context.Canceled) || len(out.results) != 2 || out.results[0].Status != Cancelled || out.results[1].Status != Cancelled || out.results[0].StartedAt.IsZero() || !out.results[1].StartedAt.IsZero() {
					t.Fatalf("outcome: %+v, %v", out.results, out.err)
				}
				for _, call := range a.calls {
					if strings.HasPrefix(call, "b:") {
						t.Fatalf("started after cancellation: %s", call)
					}
				}
			case <-time.After(5 * time.Second):
				t.Fatal("cancellation did not finish")
			}
		})
	}
}

func TestRunTaskTimeout(t *testing.T) {
	for _, phase := range []string{"precheck", "execute", "verify"} {
		t.Run(phase, func(t *testing.T) {
			task := planTask("a")
			task.Timeout = 20 * time.Millisecond
			wait := func(ctx context.Context, task Task) {
				if task.ID == "a" {
					<-ctx.Done()
				}
			}
			a := &fakeAdapter{}
			switch phase {
			case "precheck":
				a.precheck = func(ctx context.Context, task Task) (TaskStatus, error) { wait(ctx, task); return Pending, nil }
			case "execute":
				a.execute = func(ctx context.Context, task Task) (bool, error) { wait(ctx, task); return false, nil }
			case "verify":
				a.verify = func(ctx context.Context, task Task) error { wait(ctx, task); return nil }
			}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			results, err := Run(ctx, []Task{task, planTask("b", "a"), planTask("c")}, a)
			if err != nil || results[0].Status != Failed || results[0].Error != "task deadline exceeded" || results[1].Status != Skipped || results[2].Status != Success {
				t.Fatalf("results: %+v, error: %v", results, err)
			}
		})
	}
}

func TestRunParentDeadline(t *testing.T) {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	a := &fakeAdapter{}
	results, err := Run(ctx, []Task{planTask("a")}, a)
	if !errors.Is(err, context.DeadlineExceeded) || len(results) != 1 || results[0].Status != Cancelled || len(a.calls) != 0 {
		t.Fatalf("results: %+v, calls: %v, error: %v", results, a.calls, err)
	}
}
