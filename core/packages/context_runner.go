package packages

import (
	"context"
	"windeploy/core/usercontext"
)

// ContextRunner rechecks user context immediately before starting an installer,
// including after downloads/retry waits. Read-only planning is unaffected.
type ContextRunner struct {
	Next  Runner
	Check func(context.Context) error
}

func (r ContextRunner) Run(ctx context.Context, command Command, report func(string)) (Execution, error) {
	if r.Check == nil || r.Next == nil {
		return Execution{}, usercontext.ErrUnconfirmed
	}
	if err := r.Check(ctx); err != nil {
		return Execution{}, err
	}
	return r.Next.Run(ctx, command, report)
}
