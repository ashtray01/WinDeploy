package engine

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

type transientError struct{}

func (transientError) Error() string   { return "transient private diagnostic" }
func (transientError) Retryable() bool { return true }

func TestRetryPolicy(t *testing.T) {
	for _, phase := range []string{"precheck", "execute", "verify"} {
		t.Run(phase, func(t *testing.T) {
			attempts := 0
			a := &fakeAdapter{
				precheck: func(context.Context, Task) (TaskStatus, error) {
					attempts++
					if phase == "precheck" && attempts == 1 {
						return Pending, transientError{}
					}
					return Pending, nil
				},
				execute: func(context.Context, Task) (bool, error) {
					if phase == "execute" && attempts == 1 {
						return true, fmt.Errorf("wrapped: %w", transientError{})
					}
					return false, nil
				},
				verify: func(context.Context, Task) error {
					if phase == "verify" && attempts == 1 {
						return transientError{}
					}
					return nil
				},
			}
			results, err := RunWithOptions(context.Background(), []Task{planTask("a")}, a, RunOptions{MaxAttempts: 3})
			if err != nil || results[0].Status != Success || results[0].Attempts != 2 || results[0].Error != "" {
				t.Fatalf("%+v %v", results, err)
			}
			if phase == "execute" && !results[0].RebootRequired {
				t.Fatal("lost reboot requirement from failed attempt")
			}
		})
	}
}

func TestRetryLimitAndPermanentErrors(t *testing.T) {
	for _, test := range []struct {
		err         error
		limit, want int
	}{
		{transientError{}, 0, 1}, {transientError{}, 3, 3}, {errors.New("permanent"), 3, 1},
	} {
		a := &fakeAdapter{execute: func(context.Context, Task) (bool, error) { return false, test.err }}
		results, err := RunWithOptions(context.Background(), []Task{planTask("a")}, a, RunOptions{MaxAttempts: test.limit})
		if err != nil || results[0].Status != Failed || results[0].Attempts != test.want {
			t.Fatalf("%+v %v", results, err)
		}
	}
}

func TestRetryDetectsInstalledEffects(t *testing.T) {
	checks := 0
	a := &fakeAdapter{
		precheck: func(context.Context, Task) (TaskStatus, error) {
			checks++
			if checks > 1 {
				return AlreadyInstalled, nil
			}
			return Pending, nil
		},
		execute: func(context.Context, Task) (bool, error) { return false, transientError{} },
	}
	results, err := RunWithOptions(context.Background(), []Task{planTask("a")}, a, RunOptions{MaxAttempts: 2})
	if err != nil || results[0].Status != AlreadyInstalled || len(a.calls) != 3 {
		t.Fatalf("%+v %v %v", results, a.calls, err)
	}
}

func TestRetryDelayCancellationAndTimeout(t *testing.T) {
	for _, timeout := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		task := planTask("a")
		if timeout {
			task.Timeout = 20 * time.Millisecond
		}
		a := &fakeAdapter{execute: func(context.Context, Task) (bool, error) {
			if !timeout {
				cancel()
			}
			return false, transientError{}
		}}
		results, err := RunWithOptions(ctx, []Task{task}, a, RunOptions{MaxAttempts: 3, RetryDelay: time.Hour})
		cancel()
		want := Cancelled
		if timeout {
			want = Failed
			if err != nil {
				t.Fatal(err)
			}
		} else if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if results[0].Status != want || results[0].Attempts != 1 {
			t.Fatalf("%+v", results)
		}
	}
}

func TestInvalidRetryOptions(t *testing.T) {
	for _, opts := range []RunOptions{{MaxAttempts: -1}, {RetryDelay: -1}} {
		a := &fakeAdapter{}
		if _, err := RunWithOptions(context.Background(), []Task{planTask("a")}, a, opts); err == nil || len(a.calls) != 0 {
			t.Fatal("invalid retry options accepted")
		}
	}
}
