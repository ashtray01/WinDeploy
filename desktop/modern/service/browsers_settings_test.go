package service

import (
	"strings"
	"testing"

	"windeploy/core/browsers"
	"windeploy/core/settings"
)

func TestBrowserEnvironmentAndDryRun(t *testing.T) {
	service := &Service{}
	env := service.GetBrowserEnvironment()
	if env.Desktop == "" {
		t.Fatal("desktop directory was not resolved")
	}
	result, err := service.ApplyBrowserPolicy(browsers.Policy{
		Browser: browsers.Chrome, Scope: browsers.ScopeUser,
		Homepage: "https://intranet.example/", StartupURLs: []string{"https://intranet.example/"},
		Bookmarks: []browsers.Bookmark{{Name: "Портал", URL: "https://intranet.example/"}},
	}, true)
	if err != nil {
		t.Fatalf("browser dry-run failed: %v", err)
	}
	if !result.DryRun || len(result.Applied) == 0 {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
}

func TestWindowsSettingsReadAndDryRun(t *testing.T) {
	service := &Service{}
	view, err := service.GetWindowsSettings()
	if err != nil {
		t.Fatalf("settings read failed: %v", err)
	}
	if view.Windows == "" || view.Activation.Status == "" {
		t.Fatalf("incomplete settings view: %+v", view)
	}
	if options := service.GetPowerPlanOptions(); len(options) != 3 {
		t.Fatalf("unexpected power plan options: %+v", options)
	}
	rename, err := service.ApplyComputerName("WIN DEPLOY BAD", true)
	if err == nil && rename.Applied {
		t.Fatal("invalid computer name was applied")
	}
	timezone, err := service.ApplyTimezone("UTC", true)
	if err != nil {
		t.Fatalf("timezone preview failed: %v", err)
	}
	if timezone.Applied {
		t.Fatal("timezone dry-run applied changes")
	}
	showHidden := true
	explorer, err := service.ApplyExplorer(settings.ExplorerOptions{ShowHidden: &showHidden}, true)
	if err != nil {
		t.Fatalf("explorer preview failed: %v", err)
	}
	if explorer.Applied || len(explorer.Changes) == 0 {
		t.Fatalf("unexpected explorer preview: %+v", explorer)
	}
	power, err := service.ApplyPowerPlan("381b4222-f694-41f0-9685-ff5bb260df2e", true)
	if err != nil {
		t.Fatalf("power preview failed: %v", err)
	}
	if power.Applied {
		t.Fatal("power dry-run applied changes")
	}
	diagnostics, err := service.GetWindowsUpdateDiagnostics()
	if err != nil || len(diagnostics.Checks) == 0 {
		t.Fatalf("update diagnostics failed: %+v, %v", diagnostics, err)
	}
}

func TestInvalidTimezoneError(t *testing.T) {
	service := &Service{}
	if _, err := service.ApplyTimezone("Definitely Not A Zone", true); err == nil || !strings.Contains(err.Error(), "часовой пояс") {
		t.Fatalf("unexpected error: %v", err)
	}
}
