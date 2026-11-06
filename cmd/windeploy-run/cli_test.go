package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"windeploy/core/deploy"
	"windeploy/core/engine"
)

func testEnvironment(t *testing.T) environment {
	t.Helper()
	return environment{Sessions: t.TempDir(), Reports: t.TempDir(), RunOptions: engine.RunOptions{MaxAttempts: 1}}
}

func TestPlanEmbeddedProfile(t *testing.T) {
	env := testEnvironment(t)
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"plan", "--profile", "office"}, &stdout, &stderr, env)
	if code != exitOK {
		t.Fatalf("plan exit=%d stderr=%s", code, stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "sevenzip") || !strings.Contains(output, "network-check") {
		t.Fatalf("unexpected plan:\n%s", output)
	}
}

func TestPlanJSON(t *testing.T) {
	env := testEnvironment(t)
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"plan", "--profile", "office", "--json"}, &stdout, &stderr, env)
	if code != exitOK {
		t.Fatalf("json plan exit=%d stderr=%s", code, stderr.String())
	}
	var preview struct {
		Scope string `json:"scope"`
		Tasks []struct {
			ID       string `json:"id"`
			Category string `json:"category"`
		} `json:"tasks"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &preview); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, stdout.String())
	}
	if preview.Scope != "structure-and-dag-only" || len(preview.Tasks) < 2 {
		t.Fatalf("unexpected preview: %+v", preview)
	}
	for _, task := range preview.Tasks {
		if strings.Contains(task.ID, "password") || strings.Contains(task.Category, "secret") {
			t.Fatalf("preview leaked task parameters: %+v", task)
		}
	}
}

func TestRunAndResumeRequireConfirmation(t *testing.T) {
	dir := t.TempDir()
	profile := `version: 1
id: selftest
name: Self-test
components:
  - id: dns-check
    name: Проверить DNS
    category: diagnostics
    timeout: 30s
    parameters:
      dns: localhost
      dhcp: "false"
      gateways: "false"
`
	if err := os.WriteFile(filepath.Join(dir, "selftest.yaml"), []byte(profile), 0o644); err != nil {
		t.Fatal(err)
	}
	env := testEnvironment(t)
	var stdout, stderr bytes.Buffer
	code := run(context.Background(), []string{"run", "--profile", "selftest", "--profiles", dir}, &stdout, &stderr, env)
	if code != exitConfirm || !strings.Contains(stderr.String(), "--yes") {
		t.Fatalf("run without --yes: exit=%d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	code = run(context.Background(), []string{"resume", "--session", "d20260101-000000-abcd"}, &stdout, &stderr, env)
	if code != exitConfirm {
		t.Fatalf("resume without --yes: exit=%d stderr=%s", code, stderr.String())
	}
}

func TestUsageAndErrors(t *testing.T) {
	env := testEnvironment(t)
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), nil, &stdout, &stderr, env); code != exitOK || !strings.Contains(stdout.String(), "windeploy-run") {
		t.Fatalf("usage failed: %d %s", code, stdout.String())
	}
	stdout.Reset()
	if code := run(context.Background(), []string{"unknown"}, &stdout, &stderr, env); code != exitUsage {
		t.Fatalf("unknown command exit=%d", code)
	}
	if code := run(context.Background(), []string{"plan", "--profile", "missing"}, &stdout, &stderr, env); code != exitPlan {
		t.Fatalf("missing profile exit=%d", code)
	}
	sessions, err := (deploy.Store{Dir: env.Sessions}).List(context.Background())
	if err != nil || len(sessions) != 0 {
		t.Fatalf("unexpected sessions: %+v %v", sessions, err)
	}
}

func TestSessionsEmptyAndMissingReport(t *testing.T) {
	env := testEnvironment(t)
	var stdout, stderr bytes.Buffer
	if code := run(context.Background(), []string{"sessions"}, &stdout, &stderr, env); code != exitOK || !strings.Contains(stdout.String(), "Сессий нет") {
		t.Fatalf("sessions exit=%d %s", code, stdout.String())
	}
	if code := run(context.Background(), []string{"report", "--session", "d20260101-000000-abcd"}, &stdout, &stderr, env); code != exitSession {
		t.Fatalf("missing report exit=%d", code)
	}
}
