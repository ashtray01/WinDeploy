package inventory

import (
	"context"
	"errors"
	"testing"

	"windeploy/core/engine"
)

func workstation() SystemInfo {
	return SystemInfo{Architecture: "amd64", ProcessArchitecture: "386", Windows: Version{Major: 10, Build: 26100, ProductType: 1}}
}

func TestApplicabilityMatrix(t *testing.T) {
	system := workstation()
	for _, test := range []struct {
		req  engine.OSRequirements
		want bool
		bad  bool
	}{
		{engine.OSRequirements{}, true, false},
		{engine.OSRequirements{Versions: []string{"windows-11"}, Architectures: []string{"amd64"}}, true, false},
		{engine.OSRequirements{Versions: []string{"windows-7-sp1"}}, false, false},
		{engine.OSRequirements{Architectures: []string{"386"}}, false, false},
		{engine.OSRequirements{Versions: []string{"windows-11", "typo"}}, false, true},
		{engine.OSRequirements{Architectures: []string{"arm64"}}, false, true},
	} {
		got, err := Applicable(test.req, system)
		if got != test.want || (err != nil) != test.bad {
			t.Fatalf("%+v: %t %v", test.req, got, err)
		}
	}
	for _, target := range []SystemInfo{{}, {Architecture: "amd64", Windows: Version{Major: 10, Build: 26100, ProductType: 3}}, {Architecture: "386", Windows: Version{Major: 6, Minor: 1, Build: 7600, ProductType: 1}}} {
		if _, err := Applicable(engine.OSRequirements{}, target); err == nil {
			t.Fatal("unknown/unsupported target accepted")
		}
	}
	legacy := SystemInfo{Architecture: "386", Windows: Version{Major: 6, Minor: 1, Build: 7601, ServicePackMajor: 1, ProductType: 1}}
	if ok, err := Applicable(engine.OSRequirements{Versions: []string{"windows-7-sp1"}, Architectures: []string{"386"}}, legacy); err != nil || !ok {
		t.Fatal("supported legacy target rejected")
	}
}

type countingAdapter struct{ calls int }

func (a *countingAdapter) Precheck(context.Context, engine.Task) (engine.TaskStatus, error) {
	a.calls++
	return engine.Pending, nil
}
func (a *countingAdapter) Execute(context.Context, engine.Task) (bool, error) {
	a.calls++
	return false, nil
}
func (a *countingAdapter) Verify(context.Context, engine.Task) error { a.calls++; return nil }

func TestGuardPreventsIncompatibleExecution(t *testing.T) {
	next := &countingAdapter{}
	guard, err := GuardAdapter(workstation(), next)
	if err != nil {
		t.Fatal(err)
	}
	task := engine.Task{ID: "legacy", Name: "Legacy", Category: "packages", Status: engine.Pending, Requirements: engine.OSRequirements{Versions: []string{"windows-7-sp1"}}}
	dependent := engine.Task{ID: "dependent", Name: "Dependent", Category: "packages", Status: engine.Pending, Dependencies: []string{"legacy"}}
	results, err := engine.Run(context.Background(), []engine.Task{task, dependent}, guard)
	if err != nil || results[0].Status != engine.NotApplicable || results[1].Status != engine.Skipped || next.calls != 0 {
		t.Fatalf("%+v %v calls=%d", results, err, next.calls)
	}
	if _, err = guard.Execute(context.Background(), task); !errors.Is(err, ErrNotApplicable) {
		t.Fatal(err)
	}
	if err = guard.Verify(context.Background(), task); !errors.Is(err, ErrNotApplicable) {
		t.Fatal(err)
	}
	task.Requirements = engine.OSRequirements{}
	if _, err = engine.Run(context.Background(), []engine.Task{task}, guard); err != nil || next.calls != 3 {
		t.Fatalf("matching task not delegated: %v calls=%d", err, next.calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err = guard.Execute(ctx, task); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = GuardAdapter(SystemInfo{}, next); err == nil {
		t.Fatal("unknown target accepted")
	}
}
