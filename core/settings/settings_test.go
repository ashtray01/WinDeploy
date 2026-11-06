package settings

import (
	"context"
	"errors"
	"strings"
	"testing"

	"windeploy/core/inventory"
)

type fakeRegistry struct {
	strings   map[string]string
	dwords    map[string]uint32
	keys      map[string]bool
	values    map[string]bool
	writeFail error
}

func (r *fakeRegistry) key(root Root, path string) string { return string(root) + "/" + path }
func (r *fakeRegistry) value(root Root, path, name string) string {
	return r.key(root, path) + "/" + name
}
func (r *fakeRegistry) StringValue(_ context.Context, root Root, path, name string) (string, bool, error) {
	value, ok := r.strings[r.value(root, path, name)]
	return value, ok, nil
}
func (r *fakeRegistry) DWordValue(_ context.Context, root Root, path, name string) (uint32, bool, error) {
	value, ok := r.dwords[r.value(root, path, name)]
	return value, ok, nil
}
func (r *fakeRegistry) SetStringValue(_ context.Context, root Root, path, name, value string) error {
	if r.writeFail != nil {
		return r.writeFail
	}
	if r.strings == nil {
		r.strings = map[string]string{}
	}
	r.strings[r.value(root, path, name)] = value
	r.keys[r.key(root, path)] = true
	return nil
}
func (r *fakeRegistry) SetDWordValue(_ context.Context, root Root, path, name string, value uint32) error {
	if r.writeFail != nil {
		return r.writeFail
	}
	if r.dwords == nil {
		r.dwords = map[string]uint32{}
	}
	r.dwords[r.value(root, path, name)] = value
	r.keys[r.key(root, path)] = true
	return nil
}
func (r *fakeRegistry) DeleteValue(context.Context, Root, string, string) error { return nil }
func (r *fakeRegistry) KeyExists(_ context.Context, root Root, path string) (bool, error) {
	return r.keys[r.key(root, path)], nil
}
func (r *fakeRegistry) ValueExists(_ context.Context, root Root, path, name string) (bool, error) {
	if r.values != nil {
		if value, ok := r.values[r.value(root, path, name)]; ok {
			return value, nil
		}
	}
	_, ok := r.dwords[r.value(root, path, name)]
	return ok, nil
}

type fakeRunner struct {
	output []string
	calls  [][]string
}

func (r *fakeRunner) Run(_ context.Context, executable string, args ...string) (string, error) {
	r.calls = append(r.calls, append([]string{executable}, args...))
	if len(r.output) > 0 {
		value := r.output[0]
		r.output = r.output[1:]
		return value, nil
	}
	return "", nil
}

type fakeServices struct{ state string }

func (s fakeServices) State(context.Context, string) (string, error) { return s.state, nil }

type fakeElevation bool

func (e fakeElevation) Elevated(context.Context) (bool, error) { return bool(e), nil }

type fakeComputer struct{ name string }

func (c *fakeComputer) SetComputerName(_ context.Context, name string) error {
	c.name = name
	return nil
}

type fakeActivation struct{}

func (fakeActivation) Status(context.Context) (Activation, error) {
	return Activation{Genuine: true, Status: "genuine"}, nil
}

func testManager(registry Registry) *Manager {
	return &Manager{Registry: registry, Runner: &fakeRunner{}, Services: fakeServices{state: "running"}, Elevation: fakeElevation(true), Computer: &fakeComputer{}, Activation: fakeActivation{}, System: inventorySystem()}
}

func inventorySystem() inventory.SystemInfo { return inventory.SystemInfo{Hostname: "PC-01"} }

func TestPreviewComputerName(t *testing.T) {
	if _, err := PreviewComputerName("PC-01", "PC-02"); err != nil {
		t.Fatalf("valid name rejected: %v", err)
	}
	for _, name := range []string{"", "-bad", "bad name", strings.Repeat("a", 70), "PC-01"} {
		if _, err := PreviewComputerName("PC-01", name); err == nil {
			t.Errorf("invalid name %q accepted", name)
		}
	}
}

func TestSetComputerNameDryRunAndApply(t *testing.T) {
	computer := &fakeComputer{}
	manager := testManager(&fakeRegistry{keys: map[string]bool{}})
	manager.Computer = computer
	manager.System = inventorySystem()
	result, err := manager.SetComputerName(context.Background(), "PC-02", true)
	if err != nil || result.Applied || computer.name != "" {
		t.Fatalf("dry-run changed state: %+v, %v", result, err)
	}
	result, err = manager.SetComputerName(context.Background(), "PC-02", false)
	if err != nil || !result.Applied || !result.RebootRequired || computer.name != "PC-02" {
		t.Fatalf("rename failed: %+v, %v", result, err)
	}
	manager.Elevation = fakeElevation(false)
	if _, err := manager.SetComputerName(context.Background(), "PC-03", false); !errors.Is(err, ErrElevationRequired) {
		t.Fatalf("expected elevation error, got %v", err)
	}
}

func TestSetTimezone(t *testing.T) {
	registry := &fakeRegistry{keys: map[string]bool{"HKLM/SOFTWARE\\Microsoft\\Windows NT\\CurrentVersion\\Time Zones\\UTC": true}}
	runner := &fakeRunner{output: []string{"UTC\n", ""}}
	manager := testManager(registry)
	manager.Runner = runner
	result, err := manager.SetTimezone(context.Background(), "UTC", false)
	if err != nil {
		t.Fatalf("timezone failed: %v", err)
	}
	if !result.Applied || len(runner.calls) != 2 || runner.calls[1][1] != "/s" || runner.calls[1][2] != "UTC" {
		t.Fatalf("unexpected calls: %+v, result=%+v", runner.calls, result)
	}
	if _, err := manager.SetTimezone(context.Background(), "Missing Zone", false); err == nil {
		t.Fatal("unknown time zone accepted")
	}
}

func TestApplyExplorer(t *testing.T) {
	registry := &fakeRegistry{dwords: map[string]uint32{}, keys: map[string]bool{}}
	manager := testManager(registry)
	showHidden, showExtensions := true, true
	result, err := manager.ApplyExplorer(context.Background(), ExplorerOptions{ShowHidden: &showHidden, ShowExtensions: &showExtensions}, false)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if !result.Applied || len(result.Changes) != 2 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if registry.dwords["HKCU/Software\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Advanced/Hidden"] != 1 {
		t.Fatalf("Hidden not written: %+v", registry.dwords)
	}
	if registry.dwords["HKCU/Software\\Microsoft\\Windows\\CurrentVersion\\Explorer\\Advanced/HideFileExt"] != 0 {
		t.Fatalf("HideFileExt not written: %+v", registry.dwords)
	}
}

func TestSetPowerPlan(t *testing.T) {
	guid := "381b4222-f694-41f0-9685-ff5bb260df2e"
	registry := &fakeRegistry{keys: map[string]bool{"HKLM/SYSTEM\\CurrentControlSet\\Control\\Power\\User\\PowerSchemes\\" + guid: true}}
	runner := &fakeRunner{output: []string{"Power Scheme GUID: 381b4222-f694-41f0-9685-ff5bb260df2e  (Balanced)\n", ""}}
	manager := testManager(registry)
	manager.Runner = runner
	result, err := manager.SetPowerPlan(context.Background(), guid, false)
	if err != nil {
		t.Fatalf("power plan failed: %v", err)
	}
	if !result.Applied || result.Changes[0].OldValue != guid {
		t.Fatalf("unexpected result: %+v", result)
	}
	if _, err := manager.SetPowerPlan(context.Background(), "not-a-guid", false); err == nil {
		t.Fatal("invalid GUID accepted")
	}
}

func TestUpdateDiagnostics(t *testing.T) {
	registry := &fakeRegistry{keys: map[string]bool{
		"HKLM/SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Component Based Servicing\\RebootPending": true,
	}, values: map[string]bool{
		"HKLM/SYSTEM\\CurrentControlSet\\Control\\Session Manager/PendingFileRenameOperations": true,
	}}
	manager := testManager(registry)
	report, err := manager.UpdateDiagnostics(context.Background())
	if err != nil {
		t.Fatalf("diagnostics failed: %v", err)
	}
	if len(report.Checks) < 2 {
		t.Fatalf("too few checks: %+v", report)
	}
	rebootFound := false
	for _, check := range report.Checks {
		if check.Name == "Ожидается перезагрузка" && check.Status == "warning" {
			rebootFound = true
		}
	}
	if !rebootFound {
		t.Fatalf("pending reboot not detected: %+v", report.Checks)
	}
}

func TestActivationStatus(t *testing.T) {
	manager := testManager(&fakeRegistry{})
	status, err := manager.ActivationStatus(context.Background())
	if err != nil || !status.Genuine || status.Status != "genuine" {
		t.Fatalf("unexpected activation: %+v, %v", status, err)
	}
}
