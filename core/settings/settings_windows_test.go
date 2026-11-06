//go:build windows

package settings

import (
	"context"
	"testing"

	"windeploy/core/inventory"
)

func TestNativeReadOnlySettings(t *testing.T) {
	ctx := context.Background()
	manager := &Manager{
		Registry: WindowsRegistry{}, Runner: WindowsRunner{}, Services: WindowsServices{},
		Elevation: WindowsElevation{}, Activation: WindowsActivation{}, Computer: WindowsComputerName{},
		System: inventory.SystemInfo{Hostname: "TEST-PC"},
	}
	values, err := manager.ExplorerValues(ctx)
	if err != nil {
		t.Fatalf("Explorer read failed: %v", err)
	}
	_ = values
	state, err := (WindowsServices{}).State(ctx, "wuauserv")
	if err != nil || state == "" {
		t.Fatalf("update service state failed: %q, %v", state, err)
	}
	activation, err := manager.ActivationStatus(ctx)
	if err != nil {
		t.Fatalf("activation probe failed: %v", err)
	}
	if activation.Status == "" {
		t.Fatalf("activation status is empty: %+v", activation)
	}
	report, err := manager.UpdateDiagnostics(ctx)
	if err != nil || len(report.Checks) == 0 {
		t.Fatalf("update diagnostics failed: %+v, %v", report, err)
	}
}

func TestNativeTimezoneAndPowerDryRun(t *testing.T) {
	ctx := context.Background()
	manager := &Manager{
		Registry: WindowsRegistry{}, Runner: WindowsRunner{}, Elevation: WindowsElevation{},
		System: inventory.SystemInfo{Hostname: "TEST-PC"},
	}
	result, err := manager.SetTimezone(ctx, "UTC", true)
	if err != nil {
		t.Fatalf("timezone preview failed: %v", err)
	}
	if result.Applied {
		t.Fatal("timezone dry-run applied changes")
	}
	power, err := manager.SetPowerPlan(ctx, "381b4222-f694-41f0-9685-ff5bb260df2e", true)
	if err != nil {
		t.Fatalf("power plan preview failed: %v", err)
	}
	if power.Applied {
		t.Fatal("power plan dry-run applied changes")
	}
	if _, err := manager.SetTimezone(ctx, "Not A Timezone", true); err == nil {
		t.Fatal("unknown timezone accepted")
	}
}
