package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"windeploy/core/inventory"
	"windeploy/core/packages"
)

type packageFacts struct {
	mu    sync.Mutex
	apps  []inventory.InstalledApp
	files map[string]packages.FileInfo
}

func (f *packageFacts) Apps(context.Context) ([]inventory.InstalledApp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]inventory.InstalledApp{}, f.apps...), nil
}

func (f *packageFacts) File(_ context.Context, path string) (packages.FileInfo, error) {
	if info, ok := f.files[path]; ok {
		return info, nil
	}
	return packages.FileInfo{}, nil
}

func (f *packageFacts) ExpandPath(path string) string { return path }

func (f *packageFacts) addApp(app inventory.InstalledApp) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.apps = append(f.apps, app)
}

type packageRunner struct {
	mu      sync.Mutex
	calls   int
	onRun   func()
	results []packages.Execution
}

func (r *packageRunner) Run(context.Context, packages.Command, func(string)) (packages.Execution, error) {
	r.mu.Lock()
	index := r.calls
	r.calls++
	onRun := r.onRun
	var result packages.Execution
	if index < len(r.results) {
		result = r.results[index]
	}
	r.mu.Unlock()
	if onRun != nil {
		onRun()
	}
	return result, nil
}

type packageWinget struct{ available bool }

func (w packageWinget) Available(context.Context) (bool, string) { return w.available, "1.9.0" }

type packageElevation bool

func (e packageElevation) Elevated(context.Context) (bool, error) { return bool(e), nil }

func serviceTestCatalog(requiresAdmin bool) packages.Catalog {
	return packages.Catalog{Version: packages.CatalogVersion, Packages: []packages.Package{{
		ID: "testapp", Name: "Test App", Publisher: "Test Publisher", Category: "utilities",
		RequiresAdmin: requiresAdmin,
		Detections:    []packages.Detection{{Kind: "uninstall", NamePattern: "^Test App$"}},
		Sources:       []packages.Source{{ID: "winget", Kind: "winget", WingetID: "Test.App", Version: "2.0"}},
	}}}
}

func newTestServiceState(t *testing.T, catalog packages.Catalog, facts *packageFacts, runner *packageRunner, elevated bool) (*packageState, *Service) {
	t.Helper()
	deps := packageDeps{
		catalog: catalog, facts: facts, runner: runner,
		signer: &fakePackageSigner{verified: true}, elevation: packageElevation(elevated),
		winget: packageWinget{available: true}, fetcher: &fakePackageFetcher{}, cacheDir: t.TempDir(),
	}
	state := newPackageState(context.Background(), deps, func(context.Context) inventory.SystemInfo {
		return inventory.SystemInfo{Architecture: "amd64", Windows: inventory.Version{Major: 10, Minor: 0, Build: 22631, ProductType: 1}}
	})
	t.Cleanup(state.shutdown)
	return state, &Service{packages: state, userContextCheck: func(context.Context) error { return nil }}
}

type fakePackageSigner struct{ verified bool }

func (s *fakePackageSigner) Verify(context.Context, string) (packages.Signature, error) {
	return packages.Signature{Verified: s.verified, Signer: "Test Publisher", Subject: "CN=Test Publisher"}, nil
}

type fakePackageFetcher struct{}

func (f *fakePackageFetcher) Download(context.Context, packages.DownloadRequest) (packages.DownloadResult, error) {
	return packages.DownloadResult{}, fmt.Errorf("not used")
}

func installView(state *packageState) PackageInstallView {
	state.mu.Lock()
	defer state.mu.Unlock()
	return copyInstallView(state.install)
}

func waitForInstall(t *testing.T, state *packageState) PackageInstallView {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		view := installView(state)
		if !view.Running && view.Generation > 0 {
			return view
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("install did not finish: %+v", installView(state))
	return PackageInstallView{}
}

func TestPackageCatalogView(t *testing.T) {
	facts := &packageFacts{}
	_, service := newTestServiceState(t, serviceTestCatalog(true), facts, &packageRunner{}, true)
	view, err := service.GetPackages()
	if err != nil {
		t.Fatalf("catalog error: %v", err)
	}
	if len(view.Packages) != 1 {
		t.Fatalf("expected one package, got %d", len(view.Packages))
	}
	pkg := view.Packages[0]
	if pkg.Action != "install" || pkg.ActionLabel != "Установить" || pkg.RequiresAdmin != true {
		t.Fatalf("unexpected package view: %+v", pkg)
	}
	if !view.Environment.Elevated || !view.Environment.Winget || view.Environment.Windows != "Windows 11" {
		t.Fatalf("unexpected environment: %+v", view.Environment)
	}
	facts.addApp(inventory.InstalledApp{Name: "Test App", Version: "2.0"})
	view, err = service.GetPackages()
	if err != nil {
		t.Fatal(err)
	}
	if view.Packages[0].Action != "up-to-date" || !view.Packages[0].Installed {
		t.Fatalf("unexpected installed state: %+v", view.Packages[0])
	}
}

func TestStartPackageInstallCompletesAndReports(t *testing.T) {
	facts := &packageFacts{}
	runner := &packageRunner{}
	runner.onRun = func() { facts.addApp(inventory.InstalledApp{Name: "Test App", Version: "2.0"}) }
	state, service := newTestServiceState(t, serviceTestCatalog(false), facts, runner, true)
	started, err := service.StartPackageInstall([]string{"testapp"})
	if err != nil {
		t.Fatalf("start error: %v", err)
	}
	if !started.Running || started.Total != 1 || len(started.Results) != 1 {
		t.Fatalf("unexpected initial state: %+v", started)
	}
	view := waitForInstall(t, state)
	if len(view.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(view.Results))
	}
	result := view.Results[0]
	if result.Status != "success" || result.StatusLabel != "Установлено" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !strings.Contains(view.Message, "Установка завершена") {
		t.Fatalf("unexpected final message: %s", view.Message)
	}
	if len(view.Events) == 0 {
		t.Fatal("progress events were not recorded")
	}
}

func TestInstallRequiresAdministratorSynchronously(t *testing.T) {
	facts := &packageFacts{}
	_, service := newTestServiceState(t, serviceTestCatalog(true), facts, &packageRunner{}, false)
	_, err := service.StartPackageInstall([]string{"testapp"})
	if err == nil || !strings.Contains(err.Error(), "права администратора") {
		t.Fatalf("expected elevation error, got %v", err)
	}
	if view := installView(service.packages); view.Running || view.Generation != 0 {
		t.Fatalf("install must not start: %+v", view)
	}
}

func TestCancelWhenIdleIsStable(t *testing.T) {
	_, service := newTestServiceState(t, serviceTestCatalog(false), &packageFacts{}, &packageRunner{}, true)
	view := service.CancelPackageInstall()
	if view.Running || view.Generation != 0 {
		t.Fatalf("unexpected cancellation state: %+v", view)
	}
}

func TestTranslateEvent(t *testing.T) {
	cases := []struct {
		code     string
		level    string
		contains string
	}{
		{"package.download-start", "info", "Загрузка"},
		{"package.hash-verified", "info", "SHA-256"},
		{"package.hash-mismatch", "error", "не совпадает"},
		{"package.signature-failed", "error", "Подпись"},
		{"package.verify-failed", "error", "не обнаружена"},
		{"package.reboot-required", "warning", "перезагрузка"},
		{"package.unknown-code", "", ""},
	}
	for _, test := range cases {
		level, message := translateEvent(packages.Event{Code: test.code, Detail: "1.0"})
		if level != test.level || !strings.Contains(message, test.contains) {
			t.Errorf("translateEvent(%q) = %q, %q", test.code, level, message)
		}
	}
}

func TestActionAndStatusLabels(t *testing.T) {
	if actionLabel(packages.ActionInstall) != "Установить" || actionLabel(packages.ActionUpToDate) != "Уже установлено" {
		t.Fatal("unexpected action labels")
	}
	if statusLabel(packages.StatusRebootRequired) != "Установлено, нужна перезагрузка" || statusLabel(packages.StatusFailed) != "Ошибка" {
		t.Fatal("unexpected status labels")
	}
}
