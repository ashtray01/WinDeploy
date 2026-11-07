package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"windeploy/core/deploy"
	"windeploy/core/engine"
	"windeploy/core/inventory"
)

type cliRebootAdapter struct {
	executes int
	pending  bool
}

func (*cliRebootAdapter) Precheck(context.Context, engine.Task) (engine.TaskStatus, error) {
	return engine.Pending, nil
}
func (a *cliRebootAdapter) Execute(context.Context, engine.Task) (bool, error) {
	a.executes++
	a.pending = true
	return true, nil
}
func (a *cliRebootAdapter) Verify(context.Context, engine.Task) error {
	if a.pending {
		return engine.ErrRebootPending
	}
	return nil
}

func TestCLIRebootBoundaryExitAndVerifyOnlyResume(t *testing.T) {
	env := testEnvironment(t)
	adapter := &cliRebootAdapter{}
	env.Adapter = adapter
	env.ReadSystem = func(context.Context) (inventory.SystemInfo, error) {
		return inventory.SystemInfo{Architecture: "amd64", Windows: inventory.Version{Major: 10, Build: 26100, ProductType: 1}}, nil
	}
	boot := "one"
	env.ReadBootID = func(context.Context) (string, error) { return boot, nil }
	dir := t.TempDir()
	profile := "version: 1\nid: pending\nname: Pending\ncomponents:\n  - id: one\n    name: One\n    category: packages\n"
	if err := os.WriteFile(filepath.Join(dir, "pending.yaml"), []byte(profile), 0o600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := run(context.Background(), []string{"run", "--profile", "pending", "--profiles", dir, "--yes"}, &out, &stderr, env); code != exitFailed || !strings.Contains(out.String(), "успешно=0") {
		t.Fatalf("false CLI success code=%d out=%s err=%s", code, out.String(), stderr.String())
	}
	sessions, err := (deploy.Store{Dir: env.Sessions}).List(context.Background())
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions=%v %v", sessions, err)
	}
	args := []string{"resume", "--session", sessions[0].ID, "--yes"}
	if code := run(context.Background(), args, &out, &stderr, env); code != exitSession {
		t.Fatalf("same boot code=%d", code)
	}
	boot = "two"
	adapter.pending = false
	if code := run(context.Background(), args, &out, &stderr, env); code != exitOK || adapter.executes != 1 {
		t.Fatalf("verify resume code=%d executes=%d err=%s", code, adapter.executes, stderr.String())
	}
}
