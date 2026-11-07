package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"windeploy/core/inventory"
)

type unknownExecutionProvider struct{ fixtureProvider }

func (*unknownExecutionProvider) System(context.Context) (inventory.SystemInfo, error) {
	return inventory.SystemInfo{}, nil
}

func TestDeploymentExecutionIgnoresCachedTarget(t *testing.T) {
	s := newDeploymentTestService(t)
	// A previous successful preview is not authorization for a later execution.
	system, _ := s.provider.System(context.Background())
	s.state.Report = &inventory.Report{System: &system}
	s.provider = &unknownExecutionProvider{}
	if _, err := s.StartDeployment("selftest", nil); err != nil {
		t.Fatal(err)
	}
	view := waitDeployment(t, s)
	if view.Completed || view.ReportHTML != "" {
		t.Fatalf("unknown target reported success: %+v", view)
	}
	sessions, err := s.ListDeploymentSessions()
	if err != nil || len(sessions) != 0 {
		t.Fatalf("rejected start persisted execution: %+v %v", sessions, err)
	}
}

func TestResumeDeploymentRejectsFreshUnknownTarget(t *testing.T) {
	s := newDeploymentTestService(t)
	if _, err := s.StartDeployment("selftest", nil); err != nil {
		t.Fatal(err)
	}
	completed := waitDeployment(t, s)
	s.provider = &unknownExecutionProvider{}
	if _, err := s.ResumeDeployment(completed.SessionID, true); err != nil {
		t.Fatal(err)
	}
	view := waitDeployment(t, s)
	if view.Completed {
		t.Fatalf("unknown resume target reported success: %+v", view)
	}
}

const deploymentProfile = `version: 1
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

func newDeploymentTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "selftest.yaml"), []byte(deploymentProfile), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("WINDEPLOY_STATE_DIR", t.TempDir())
	service := NewWithProfiles(&fixtureProvider{}, os.DirFS(dir))
	t.Cleanup(func() { _ = service.ServiceShutdown() })
	return service
}

func waitDeployment(t *testing.T, service *Service) DeploymentView {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		view := service.GetDeployment()
		if !view.Running && view.SessionID != "" {
			return view
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("deployment did not finish: %+v", service.GetDeployment())
	return DeploymentView{}
}

func TestStartDeploymentExecutesAndReports(t *testing.T) {
	service := newDeploymentTestService(t)
	view, err := service.StartDeployment("selftest", nil)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}
	if !view.Running || len(view.Tasks) != 1 {
		t.Fatalf("unexpected initial view: %+v", view)
	}
	final := waitDeployment(t, service)
	if !final.Completed || final.Cancelled {
		t.Fatalf("unexpected final view: %+v", final)
	}
	if len(final.Tasks) != 1 || final.Tasks[0].Status != "Success" {
		t.Fatalf("unexpected tasks: %+v", final.Tasks)
	}
	if final.ReportHTML == "" || final.ReportJSON == "" {
		t.Fatalf("reports missing: %+v", final)
	}
	if _, err := os.Stat(final.ReportHTML); err != nil {
		t.Fatalf("HTML report missing: %v", err)
	}
	sessions, err := service.ListDeploymentSessions()
	if err != nil || len(sessions) == 0 {
		t.Fatalf("sessions missing: %+v, %v", sessions, err)
	}
	stored, err := service.GetDeploymentSession(final.SessionID)
	if err != nil || len(stored.Tasks) != 1 || stored.Tasks[0].Status != "Success" {
		t.Fatalf("stored session mismatch: %+v, %v", stored, err)
	}
	if _, err := service.ResumeDeployment(final.SessionID, false); err == nil {
		t.Fatal("completed session resumed without retry consent")
	}
}

func TestStartDeploymentRejectsUnknownProfile(t *testing.T) {
	service := newDeploymentTestService(t)
	if _, err := service.StartDeployment("missing", nil); err == nil {
		t.Fatal("unknown profile accepted")
	}
}
