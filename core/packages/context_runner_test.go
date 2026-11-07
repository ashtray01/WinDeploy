package packages

import (
	"context"
	"errors"
	"testing"
	"windeploy/core/usercontext"
)

type countedContextRunner struct{ calls int }

func (r *countedContextRunner) Run(context.Context, Command, func(string)) (Execution, error) {
	r.calls++
	return Execution{ExitCode: 0}, nil
}
func TestContextRunnerChecksAtInstallerBoundary(t *testing.T) {
	next := &countedContextRunner{}
	allowed := true
	runner := ContextRunner{Next: next, Check: func(context.Context) error {
		if !allowed {
			return usercontext.ErrUnconfirmed
		}
		return nil
	}}
	// Context can change while the caller is preparing/downloading an artifact.
	allowed = false
	if _, err := runner.Run(context.Background(), Command{}, nil); !errors.Is(err, usercontext.ErrUnconfirmed) || next.calls != 0 {
		t.Fatalf("installer ran in wrong context: calls=%d err=%v", next.calls, err)
	}
	allowed = true
	if _, err := runner.Run(context.Background(), Command{}, nil); err != nil || next.calls != 1 {
		t.Fatalf("matching context rejected: %v", err)
	}
}
