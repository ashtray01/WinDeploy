package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"windeploy/core/browsers"
	"windeploy/core/engine"
	"windeploy/core/inventory"
	"windeploy/core/printers"
	"windeploy/core/settings"
)

// These tests cover read-back verification for Windows settings and printers: an
// applied change is only accepted when reading the system confirms it.

type readbackRegistry struct {
	strings map[string]string
	dwords  map[string]uint32
	keys    map[string]bool
}

func registryKey(root settings.Root, path string) string { return string(root) + "/" + path }
func registryValue(root settings.Root, path, name string) string {
	return registryKey(root, path) + "/" + name
}

func (r *readbackRegistry) StringValue(_ context.Context, root settings.Root, path, name string) (string, bool, error) {
	value, ok := r.strings[registryValue(root, path, name)]
	return value, ok, nil
}
func (r *readbackRegistry) DWordValue(_ context.Context, root settings.Root, path, name string) (uint32, bool, error) {
	value, ok := r.dwords[registryValue(root, path, name)]
	return value, ok, nil
}
func (r *readbackRegistry) SetStringValue(_ context.Context, root settings.Root, path, name, value string) error {
	if r.strings == nil {
		r.strings = map[string]string{}
	}
	r.strings[registryValue(root, path, name)] = value
	return nil
}
func (r *readbackRegistry) SetDWordValue(_ context.Context, root settings.Root, path, name string, value uint32) error {
	if r.dwords == nil {
		r.dwords = map[string]uint32{}
	}
	r.dwords[registryValue(root, path, name)] = value
	return nil
}
func (r *readbackRegistry) DeleteValue(context.Context, settings.Root, string, string) error {
	return nil
}
func (r *readbackRegistry) KeyExists(_ context.Context, root settings.Root, path string) (bool, error) {
	return r.keys[registryKey(root, path)], nil
}
func (r *readbackRegistry) ValueExists(_ context.Context, root settings.Root, path, name string) (bool, error) {
	_, ok := r.dwords[registryValue(root, path, name)]
	return ok, nil
}

// readbackRunner answers fixed commands and records invocations.
type readbackRunner struct {
	timezone   string
	powerPlan  string
	failRead   bool
	lastArgs   []string
	callCounts int
}

func (r *readbackRunner) Run(_ context.Context, executable string, args ...string) (string, error) {
	r.callCounts++
	r.lastArgs = append([]string{executable}, args...)
	switch {
	case executable == "tzutil.exe" && len(args) > 0 && args[0] == "/g":
		if r.failRead {
			return "", errors.New("unavailable")
		}
		return r.timezone + "\n", nil
	case executable == "powercfg.exe" && len(args) > 0 && args[0] == "/getactivescheme":
		if r.failRead {
			return "", errors.New("unavailable")
		}
		return "Power Scheme GUID: " + r.powerPlan + "  (Balanced)\n", nil
	}
	return "", nil
}

func readbackSettings(t *testing.T, runner *readbackRunner) *settings.Manager {
	t.Helper()
	registry := &readbackRegistry{
		dwords: map[string]uint32{},
		keys:   map[string]bool{},
	}
	return &settings.Manager{Registry: registry, Runner: runner, System: inventory.SystemInfo{Hostname: "PC-01"}}
}

func TestWindowsTimezoneReadBack(t *testing.T) {
	registry := &readbackRegistry{keys: map[string]bool{registryKey(settings.RootMachine, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Time Zones\UTC`): true}, dwords: map[string]uint32{}}
	runner := &readbackRunner{timezone: "UTC", powerPlan: "381b4222-f694-41f0-9685-ff5bb260df2e"}
	manager := &settings.Manager{Registry: registry, Runner: runner, System: inventory.SystemInfo{Hostname: "PC-01"}}
	adapter := &Adapter{Settings: manager, CheckUserContext: func(context.Context) error { return nil }}
	task := engine.Task{ID: "tz", Name: "Timezone", Category: "windows", Status: engine.Pending, Parameters: map[string]string{"setting": "timezone", "timezone": "UTC"}}
	results, err := engine.Run(context.Background(), []engine.Task{task}, adapter)
	if err != nil || results[0].Status != engine.Success {
		t.Fatalf("confirmed timezone rejected: %+v %v", results, err)
	}
	if len(results[0].Warnings) != 0 {
		t.Fatalf("confirmed timezone produced warnings: %+v", results[0].Warnings)
	}
}

func TestWindowsTimezoneMismatchFailsVerification(t *testing.T) {
	registry := &readbackRegistry{keys: map[string]bool{registryKey(settings.RootMachine, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Time Zones\UTC`): true}, dwords: map[string]uint32{}}
	runner := &readbackRunner{timezone: "Pacific Standard Time", powerPlan: ""}
	manager := &settings.Manager{Registry: registry, Runner: runner, System: inventory.SystemInfo{Hostname: "PC-01"}}
	adapter := &Adapter{Settings: manager, CheckUserContext: func(context.Context) error { return nil }}
	task := engine.Task{ID: "tz", Name: "Timezone", Category: "windows", Status: engine.Pending, Parameters: map[string]string{"setting": "timezone", "timezone": "UTC"}}
	results, err := engine.Run(context.Background(), []engine.Task{task}, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != engine.Failed {
		t.Fatalf("unconfirmed timezone reported success: %+v", results[0])
	}
	if len(results[0].Warnings) == 0 {
		t.Fatalf("unconfirmed timezone lost its warning: %+v", results[0])
	}
}

func TestWindowsPowerPlanReadBack(t *testing.T) {
	guid := "381b4222-f694-41f0-9685-ff5bb260df2e"
	registry := &readbackRegistry{keys: map[string]bool{registryKey(settings.RootMachine, `SYSTEM\CurrentControlSet\Control\Power\User\PowerSchemes\`+guid): true}, dwords: map[string]uint32{}}
	runner := &readbackRunner{powerPlan: guid}
	adapter := &Adapter{Settings: &settings.Manager{Registry: registry, Runner: runner}, CheckUserContext: func(context.Context) error { return nil }}
	task := engine.Task{ID: "power", Name: "Power", Category: "windows", Status: engine.Pending, Parameters: map[string]string{"setting": "power-plan", "guid": guid}}
	results, err := engine.Run(context.Background(), []engine.Task{task}, adapter)
	if err != nil || results[0].Status != engine.Success {
		t.Fatalf("confirmed power plan rejected: %+v %v", results, err)
	}
	runner.powerPlan = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
	results, err = engine.Run(context.Background(), []engine.Task{task}, &Adapter{Settings: &settings.Manager{Registry: registry, Runner: runner}, CheckUserContext: func(context.Context) error { return nil }})
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != engine.Failed {
		t.Fatalf("wrong power plan reported success: %+v", results[0])
	}
}

func TestWindowsExplorerReadBack(t *testing.T) {
	registry := &readbackRegistry{dwords: map[string]uint32{}, keys: map[string]bool{}}
	adapter := &Adapter{Settings: &settings.Manager{Registry: registry}, CheckUserContext: func(context.Context) error { return nil }}
	task := engine.Task{ID: "explorer", Name: "Explorer", Category: "windows", Status: engine.Pending, Parameters: map[string]string{"setting": "explorer", "show_hidden": "true"}}
	results, err := engine.Run(context.Background(), []engine.Task{task}, adapter)
	if err != nil || results[0].Status != engine.Success {
		t.Fatalf("confirmed Explorer value rejected: %+v %v", results, err)
	}
	// The value never appears, like a policy removed by another management tool.
	empty := &readbackRegistry{dwords: map[string]uint32{}, keys: map[string]bool{}}
	adapter = &Adapter{Settings: &settings.Manager{Registry: ignoringDWordRegistry{readbackRegistry: empty}}, CheckUserContext: func(context.Context) error { return nil }}
	results, err = engine.Run(context.Background(), []engine.Task{task}, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != engine.Failed || len(results[0].Warnings) == 0 {
		t.Fatalf("unconfirmed Explorer value reported success: %+v", results[0])
	}
}

// ignoringDWordRegistry accepts DWORD writes without storing them.
type ignoringDWordRegistry struct{ *readbackRegistry }

func (ignoringDWordRegistry) SetDWordValue(context.Context, settings.Root, string, string, uint32) error {
	return nil
}

type mutableDefaults struct{ current string }

func (m *mutableDefaults) Default(context.Context) (string, error) { return m.current, nil }
func (m *mutableDefaults) SetDefault(_ context.Context, name string) error {
	m.current = name
	return nil
}

type staticPrinterProvider struct{ items []printers.Printer }

func (p *staticPrinterProvider) Printers(context.Context) ([]printers.Printer, error) {
	return append([]printers.Printer{}, p.items...), nil
}

type noopPrinterInstaller struct{ provider *staticPrinterProvider }

func (i noopPrinterInstaller) AddPrinter(_ context.Context, request printers.AddRequest) error {
	i.provider.items = append(i.provider.items, printers.Printer{Name: request.Name, Port: "IP_" + request.IPAddress, Driver: request.Model, Status: "ok"})
	return nil
}

type allowElevation struct{}

func (allowElevation) Elevated(context.Context) (bool, error) { return true, nil }

func printerReadbackAdapter(t *testing.T, defaults printers.DefaultManager) (*Adapter, *staticPrinterProvider, string) {
	t.Helper()
	dir := t.TempDir()
	inf := filepath.Join(dir, "printer.inf")
	if err := os.WriteFile(inf, []byte("[Version]\nSignature=\"$Windows NT$\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	provider := &staticPrinterProvider{}
	manager := &printers.Manager{Printers: provider, Installer: noopPrinterInstaller{provider: provider}, Defaults: defaults, Elevation: allowElevation{}}
	return &Adapter{Printers: manager, CheckUserContext: func(context.Context) error { return nil }}, provider, inf
}

func TestPrinterVerifyRejectsUnexpectedPort(t *testing.T) {
	adapter, provider, inf := printerReadbackAdapter(t, &mutableDefaults{})
	provider.items = []printers.Printer{{Name: "Office", Port: "WSD-1234", Status: "ok"}}
	task := engine.Task{ID: "printer", Name: "Printer", Category: "printers", Status: engine.Pending, Parameters: map[string]string{
		"name": "Office", "inf_path": inf, "model": "Generic / Text Only", "ip_address": "192.168.1.50",
	}}
	if err := adapter.verifyPrinter(context.Background(), task); err == nil {
		t.Fatal("printer on an unexpected port verified")
	}
}

func TestPrinterVerifyWarnsWhenDefaultIsNotConfirmed(t *testing.T) {
	defaults := &mutableDefaults{current: "Another Printer"}
	adapter, provider, inf := printerReadbackAdapter(t, defaults)
	provider.items = []printers.Printer{{Name: "Office", Port: "IP_192.168.1.50", Status: "ok"}}
	task := engine.Task{ID: "printer", Name: "Printer", Category: "printers", Status: engine.Pending, Parameters: map[string]string{
		"name": "Office", "inf_path": inf, "model": "Generic / Text Only", "ip_address": "192.168.1.50", "set_default": "true",
	}}
	if err := adapter.verifyPrinter(context.Background(), task); err != nil {
		t.Fatalf("confirmed queue failed verification: %v", err)
	}
	warnings := adapter.TaskWarnings(task)
	if len(warnings) == 0 || !strings.Contains(strings.Join(warnings, " "), "по умолчанию") {
		t.Fatalf("unconfirmed default not reported: %+v", warnings)
	}
}

func TestAdapterKeepsBrowserWarningsFromReadBack(t *testing.T) {
	adapter := &Adapter{Browsers: &browsers.Manager{Store: unconfirmedBrowserStore{}}, CheckUserContext: func(context.Context) error { return nil }}
	task := engine.Task{ID: "browser", Name: "Browser", Category: "browsers", Status: engine.Pending, Parameters: map[string]string{"browser": "chrome", "scope": "user", "homepage": "https://example.test/"}}
	results, err := engine.Run(context.Background(), []engine.Task{task}, adapter)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].Status != engine.Failed || len(results[0].Warnings) == 0 {
		t.Fatalf("unconfirmed browser policy reported success: %+v", results[0])
	}
}

// unconfirmedBrowserStore writes values but reads back a different one.
type unconfirmedBrowserStore struct{}

func (unconfirmedBrowserStore) WriteString(context.Context, browsers.Scope, browsers.Browser, string, string) error {
	return nil
}
func (unconfirmedBrowserStore) WriteDWord(context.Context, browsers.Scope, browsers.Browser, string, uint32) error {
	return nil
}
func (unconfirmedBrowserStore) DeleteValue(context.Context, browsers.Scope, browsers.Browser, string) error {
	return nil
}
func (unconfirmedBrowserStore) ReadString(context.Context, browsers.Scope, browsers.Browser, string) (string, bool, error) {
	return "other", true, nil
}
func (unconfirmedBrowserStore) ReadDWord(context.Context, browsers.Scope, browsers.Browser, string) (uint32, bool, error) {
	return 0, true, nil
}
