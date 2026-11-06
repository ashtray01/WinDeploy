package inventory

import (
	"context"
	"errors"

	"windeploy/core/engine"
)

var ErrNotApplicable = errors.New("task requirements do not match the target")

// Applicable matches canonical profile tokens against the native OS, not the
// process architecture. Unknown targets/requirements fail closed. Empty lists
// allow the supported workstation matrix only; actual package version constraints
// must still be supplied by the package catalog/adapter.
func Applicable(requirements engine.OSRequirements, system SystemInfo) (bool, error) {
	version := system.Windows.Token()
	if version == "" || (system.Architecture != "386" && system.Architecture != "amd64") {
		return false, errors.New("target OS or architecture is unsupported or unknown")
	}
	versionMatches := len(requirements.Versions) == 0
	for _, allowed := range requirements.Versions {
		switch allowed {
		case "windows-7-sp1", "windows-10", "windows-11":
		default:
			return false, errors.New("unknown OS requirement")
		}
		if allowed == version {
			versionMatches = true
		}
	}
	archMatches := len(requirements.Architectures) == 0
	for _, allowed := range requirements.Architectures {
		if allowed != "386" && allowed != "amd64" {
			return false, errors.New("unknown architecture requirement")
		}
		if allowed == system.Architecture {
			archMatches = true
		}
	}
	return versionMatches && archMatches, nil
}

type guardedAdapter struct {
	system SystemInfo
	next   engine.Adapter
}

// GuardAdapter wraps all three phases, so a mismatched task cannot reach the real
// adapter even if Execute/Verify are called directly. The target is an immutable
// value snapshot; callers should obtain it with Native.System for actual execution.
func GuardAdapter(system SystemInfo, next engine.Adapter) (engine.Adapter, error) {
	if next == nil {
		return nil, errors.New("task adapter is required")
	}
	if _, err := Applicable(engine.OSRequirements{}, system); err != nil {
		return nil, err
	}
	return guardedAdapter{system: system, next: next}, nil
}

func (a guardedAdapter) check(ctx context.Context, task engine.Task) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	ok, err := Applicable(task.Requirements, a.system)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotApplicable
	}
	return nil
}

func (a guardedAdapter) Precheck(ctx context.Context, task engine.Task) (engine.TaskStatus, error) {
	if err := a.check(ctx, task); err != nil {
		if errors.Is(err, ErrNotApplicable) {
			return engine.NotApplicable, nil
		}
		return engine.Pending, err
	}
	return a.next.Precheck(ctx, task)
}
func (a guardedAdapter) Execute(ctx context.Context, task engine.Task) (bool, error) {
	if err := a.check(ctx, task); err != nil {
		return false, err
	}
	return a.next.Execute(ctx, task)
}
func (a guardedAdapter) Verify(ctx context.Context, task engine.Task) error {
	if err := a.check(ctx, task); err != nil {
		return err
	}
	return a.next.Verify(ctx, task)
}
