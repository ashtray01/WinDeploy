package printers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type fakeProvider struct {
	items []Printer
	err   error
	calls int
}

func (f *fakeProvider) Printers(context.Context) ([]Printer, error) {
	f.calls++
	return append([]Printer{}, f.items...), f.err
}

type fakeInstaller struct {
	calls     int
	request   AddRequest
	err       error
	onInstall func()
}

func (f *fakeInstaller) AddPrinter(_ context.Context, request AddRequest) error {
	f.calls++
	f.request = request
	if f.onInstall != nil {
		f.onInstall()
	}
	return f.err
}

type fakeDefaults struct {
	defaultName string
	setCalls    []string
	setErr      error
}

func (f *fakeDefaults) Default(context.Context) (string, error) { return f.defaultName, nil }
func (f *fakeDefaults) SetDefault(_ context.Context, name string) error {
	f.setCalls = append(f.setCalls, name)
	return f.setErr
}

type fakeTestPrinter struct {
	calls []string
	err   error
}

func (f *fakeTestPrinter) PrintTestPage(_ context.Context, name string) error {
	f.calls = append(f.calls, name)
	return f.err
}

type fakeElevation bool

func (e fakeElevation) Elevated(context.Context) (bool, error) { return bool(e), nil }

func testINF(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "printer.inf")
	if err := os.WriteFile(path, []byte("[Version]\nSignature=\"$Windows NT$\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func validRequest(t *testing.T) AddRequest {
	return AddRequest{Name: "Office Printer", INFPath: testINF(t), Model: "Generic / Text Only", IPAddress: "192.168.1.50"}
}

func TestValidateAddRequest(t *testing.T) {
	request := validRequest(t)
	if err := ValidateAddRequest(request); err != nil {
		t.Fatalf("valid request rejected: %v", err)
	}
	badIP := request
	badIP.IPAddress = "not-an-ip"
	if err := ValidateAddRequest(badIP); err == nil {
		t.Fatal("invalid IP accepted")
	}
	badINF := request
	badINF.INFPath = filepath.Join(t.TempDir(), "missing.inf")
	if err := ValidateAddRequest(badINF); err == nil {
		t.Fatal("missing INF accepted")
	}
	noModel := request
	noModel.Model = ""
	if err := ValidateAddRequest(noModel); err == nil {
		t.Fatal("missing model accepted")
	}
	shared := request
	shared.Share = true
	if err := ValidateAddRequest(shared); err == nil {
		t.Fatal("shared printer without a share name accepted")
	}
}

func TestKyoceraModelMustBeExplicit(t *testing.T) {
	request := validRequest(t)
	request.Name = "Kyocera 2460 принтер"
	request.Model = ""
	if err := ValidateAddRequest(request); !errors.Is(err, ErrModelRequired) {
		t.Fatalf("expected model requirement, got %v", err)
	}
	request.Model = "Some Other Model"
	if err := ValidateAddRequest(request); !errors.Is(err, ErrModelRequired) {
		t.Fatalf("expected model rejection, got %v", err)
	}
	request.Model = "TASKalfa 2460ci"
	if err := ValidateAddRequest(request); err != nil {
		t.Fatalf("catalogued model rejected: %v", err)
	}
	if !IsKyocera2460(request) {
		t.Fatal("Kyocera 2460 request was not detected")
	}
}

func TestAddPrinterInstallsAndConfirms(t *testing.T) {
	request := validRequest(t)
	provider := &fakeProvider{}
	installer := &fakeInstaller{onInstall: func() {
		provider.items = append(provider.items, Printer{Name: request.Name, Driver: request.Model, Port: "IP_" + request.IPAddress, Status: "ok"})
	}}
	defaults := &fakeDefaults{}
	manager := &Manager{Printers: provider, Installer: installer, Defaults: defaults, Elevation: fakeElevation(true)}
	result, err := manager.Add(context.Background(), request)
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if !result.Installed || result.Printer == nil || result.Printer.Name != request.Name {
		t.Fatalf("unexpected result: %+v", result)
	}
	if installer.calls != 1 || len(defaults.setCalls) != 0 {
		t.Fatalf("unexpected calls: installer=%d defaults=%v", installer.calls, defaults.setCalls)
	}
}

func TestAddPrinterSetsDefault(t *testing.T) {
	request := validRequest(t)
	request.SetDefault = true
	provider := &fakeProvider{}
	installer := &fakeInstaller{onInstall: func() { provider.items = []Printer{{Name: request.Name, Status: "ok"}} }}
	defaults := &fakeDefaults{}
	manager := &Manager{Printers: provider, Installer: installer, Defaults: defaults, Elevation: fakeElevation(true)}
	result, err := manager.Add(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Default || len(defaults.setCalls) != 1 || defaults.setCalls[0] != request.Name {
		t.Fatalf("default was not set: %+v, %v", result, defaults.setCalls)
	}
}

func TestAddPrinterRequiresElevationAndConfirmation(t *testing.T) {
	request := validRequest(t)
	installer := &fakeInstaller{}
	manager := &Manager{Printers: &fakeProvider{}, Installer: installer, Elevation: fakeElevation(false)}
	if _, err := manager.Add(context.Background(), request); !errors.Is(err, ErrElevationRequired) {
		t.Fatalf("expected elevation error, got %v", err)
	}
	if installer.calls != 0 {
		t.Fatal("installer must not run without elevation")
	}
	manager.Elevation = fakeElevation(true)
	if _, err := manager.Add(context.Background(), request); err == nil {
		t.Fatal("unconfirmed printer must fail")
	}
}

func TestAddPrinterDryRun(t *testing.T) {
	request := validRequest(t)
	request.DryRun = true
	installer := &fakeInstaller{}
	manager := &Manager{Printers: &fakeProvider{}, Installer: installer, Elevation: fakeElevation(false)}
	result, err := manager.Add(context.Background(), request)
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if installer.calls != 0 || result.Installed {
		t.Fatalf("dry-run changed state: %+v", result)
	}
}

func TestSetDefaultAndTestPageOnlyForExistingPrinters(t *testing.T) {
	provider := &fakeProvider{items: []Printer{{Name: "Office", Status: "ok"}}}
	defaults := &fakeDefaults{}
	tester := &fakeTestPrinter{}
	manager := &Manager{Printers: provider, Defaults: defaults, Tester: tester}
	if err := manager.SetDefault(context.Background(), "Office"); err != nil || len(defaults.setCalls) != 1 {
		t.Fatalf("set default failed: %v %v", err, defaults.setCalls)
	}
	if err := manager.SetDefault(context.Background(), "Missing"); err == nil {
		t.Fatal("missing printer accepted as default")
	}
	if err := manager.TestPage(context.Background(), "Office"); err != nil || len(tester.calls) != 1 {
		t.Fatalf("test page failed: %v %v", err, tester.calls)
	}
	if err := manager.TestPage(context.Background(), "Missing"); err == nil {
		t.Fatal("test page for missing printer accepted")
	}
}
