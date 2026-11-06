package packages

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"windeploy/core/engine"
	"windeploy/core/inventory"
)

type fakeFacts struct {
	mu      sync.Mutex
	apps    []inventory.InstalledApp
	appsErr error
	files   map[string]FileInfo
	env     map[string]string
}

func (f *fakeFacts) Apps(context.Context) ([]inventory.InstalledApp, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]inventory.InstalledApp{}, f.apps...), f.appsErr
}

func (f *fakeFacts) File(_ context.Context, path string) (FileInfo, error) {
	expanded := f.ExpandPath(path)
	if info, ok := f.files[expanded]; ok {
		return info, nil
	}
	if strings.ContainsAny(expanded, "*?") {
		matches, err := filepath.Glob(expanded)
		if err != nil {
			return FileInfo{}, err
		}
		for _, match := range matches {
			if stat, err := os.Stat(match); err == nil && stat.Mode().IsRegular() {
				return FileInfo{Exists: true}, nil
			}
		}
		return FileInfo{}, nil
	}
	if stat, err := os.Stat(expanded); err == nil && stat.Mode().IsRegular() {
		return FileInfo{Exists: true}, nil
	}
	return FileInfo{}, nil
}

func (f *fakeFacts) ExpandPath(path string) string {
	for key, value := range f.env {
		path = strings.ReplaceAll(path, "%"+key+"%", value)
	}
	return path
}

func (f *fakeFacts) setApps(apps ...inventory.InstalledApp) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.apps = append([]inventory.InstalledApp{}, apps...)
}

func (f *fakeFacts) addApp(app inventory.InstalledApp) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.apps = append(f.apps, app)
}

type fakeWinget struct {
	available bool
	version   string
	err       error
}

func (w fakeWinget) Available(context.Context) (bool, string) {
	return w.available, w.version
}

type fakeRunner struct {
	mu      sync.Mutex
	calls   []Command
	results []Execution
	errs    []error
	index   int
	onRun   func(Command)
}

func (r *fakeRunner) Run(_ context.Context, command Command, _ func(string)) (Execution, error) {
	r.mu.Lock()
	r.calls = append(r.calls, command)
	index := r.index
	r.index++
	onRun := r.onRun
	var result Execution
	var err error
	if index < len(r.results) {
		result = r.results[index]
	}
	if index < len(r.errs) {
		err = r.errs[index]
	}
	r.mu.Unlock()
	if onRun != nil {
		onRun(command)
	}
	return result, err
}

func (r *fakeRunner) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

type fakeFetcher struct {
	mu     sync.Mutex
	count  int
	data   []byte
	err    error
	onCall func(DownloadRequest)
}

func (f *fakeFetcher) Download(_ context.Context, request DownloadRequest) (DownloadResult, error) {
	f.mu.Lock()
	f.count++
	err := f.err
	data := append([]byte{}, f.data...)
	onCall := f.onCall
	f.mu.Unlock()
	if onCall != nil {
		onCall(request)
	}
	if err != nil {
		return DownloadResult{}, err
	}
	if dir := filepath.Dir(request.Destination); dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return DownloadResult{}, err
		}
	}
	if err := os.WriteFile(request.Destination, data, 0o644); err != nil {
		return DownloadResult{}, err
	}
	digest := HashBytes(data)
	if request.ExpectedSHA256 != "" && !strings.EqualFold(digest, request.ExpectedSHA256) {
		return DownloadResult{}, ErrHashMismatch
	}
	return DownloadResult{Path: request.Destination, Size: int64(len(data)), SHA256: digest}, nil
}

type fakeSigner struct {
	verified bool
	signer   string
	subject  string
	err      error
	calls    int
}

func (s *fakeSigner) Verify(context.Context, string) (Signature, error) {
	s.calls++
	return Signature{Verified: s.verified, Signer: s.signer, Subject: s.subject}, s.err
}

type fakeElevation struct {
	elevated bool
	err      error
}

func (e fakeElevation) Elevated(context.Context) (bool, error) { return e.elevated, e.err }

func testSystem() inventory.SystemInfo {
	return inventory.SystemInfo{Architecture: "amd64", Windows: inventory.Version{Major: 10, Minor: 0, Build: 22631, ProductType: 1}}
}

func testPackage() Package {
	return Package{
		ID: "test-app", Name: "Test App", Publisher: "Test Publisher", Category: "utilities",
		Detections: []Detection{{Kind: "uninstall", NamePattern: "^Test App$"}},
		Sources: []Source{{
			ID: "winget-primary", Kind: "winget", WingetID: "Test.App", Version: "2.0", Priority: 0,
		}},
	}
}

func testManager(pkg Package, facts *fakeFacts) (*Manager, *fakeRunner, *fakeFetcher) {
	runner := &fakeRunner{}
	fetcher := &fakeFetcher{data: []byte("installer")}
	manager := &Manager{
		Catalog:   Catalog{Version: CatalogVersion, Packages: []Package{pkg}},
		Facts:     facts,
		System:    testSystem(),
		Winget:    fakeWinget{available: true, version: "1.9.0"},
		Fetcher:   fetcher,
		Runner:    runner,
		Signer:    &fakeSigner{verified: true, signer: "Test Publisher", subject: "CN=Test Publisher"},
		Elevation: fakeElevation{elevated: true},
		CacheDir:  "",
	}
	return manager, runner, fetcher
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		left, right string
		want        int
	}{
		{"1.2.3", "1.2.3", 0},
		{"1.2", "1.2.0", 0},
		{"1.2.10", "1.2.9", 1},
		{"24.08", "26.05", -1},
		{"v1.0", "1.0", 0},
		{"1.0.0-beta", "1.0.0", -1},
		{"1.0.0-rc1", "1.0.0-beta", 1},
		{"", "1.0", -1},
		{"157.0.8103.39", "157.0.8103.39", 0},
		{"160.0.1", "159.9.9", 1},
	}
	for _, test := range cases {
		if got := CompareVersions(test.left, test.right); got != test.want {
			t.Errorf("CompareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
		}
	}
	if !VersionAtLeast("2.0", "1.9") || VersionAtLeast("1.0", "1.0.1") {
		t.Fatal("VersionAtLeast returned an unexpected result")
	}
	if VersionAtLeast("", "1.0") || VersionAtLeast("1.0", "") {
		t.Fatal("empty versions must not satisfy a requirement")
	}
}

func TestCatalogValidation(t *testing.T) {
	valid := testPackage()
	if err := (Catalog{Version: CatalogVersion, Packages: []Package{valid}}).Validate(); err != nil {
		t.Fatalf("valid catalog rejected: %v", err)
	}
	duplicate := valid
	duplicate.Name = "Other"
	if err := (Catalog{Version: CatalogVersion, Packages: []Package{valid, duplicate}}).Validate(); err == nil {
		t.Fatal("duplicate package IDs must be rejected")
	}
	noDetection := valid
	noDetection.Detections = nil
	if err := noDetection.Validate(); err == nil {
		t.Fatal("package without detection must be rejected")
	}
	badHTTP := valid
	badHTTP.Sources = []Source{{ID: "http", Kind: "http", URL: "http://example.test/setup.exe", Type: "exe", SHA256: strings.Repeat("a", 64)}}
	if err := badHTTP.Validate(); err == nil {
		t.Fatal("http source without TLS must be rejected")
	}
	noHash := valid
	noHash.Sources = []Source{{ID: "http", Kind: "http", URL: "https://example.test/setup.exe", Type: "exe"}}
	if err := noHash.Validate(); err == nil {
		t.Fatal("http source without a pinned digest must be rejected")
	}
	signatureMode := valid
	signatureMode.Sources = []Source{{ID: "http", Kind: "http", URL: "https://example.test/setup.exe", Type: "exe", Integrity: "signature", Signer: "Example Corp"}}
	if err := signatureMode.Validate(); err != nil {
		t.Fatalf("signature integrity source rejected: %v", err)
	}
	badSecret := valid
	badSecret.Sources = []Source{{ID: "http", Kind: "http", URL: "https://user:pass@example.test/setup.exe", Type: "exe", SHA256: strings.Repeat("a", 64)}}
	if err := badSecret.Validate(); err == nil {
		t.Fatal("URL credentials must be rejected")
	}
}

func TestDetectUninstallAndFile(t *testing.T) {
	ctx := context.Background()
	pkg := testPackage()
	facts := &fakeFacts{}
	state, err := Detect(ctx, facts, pkg)
	if err != nil {
		t.Fatalf("detection error: %v", err)
	}
	if state.Installed {
		t.Fatal("package must not be reported installed on an empty system")
	}
	facts.setApps(inventory.InstalledApp{Name: "Test App", Version: "1.2.3", Publisher: "Test Publisher"})
	state, err = Detect(ctx, facts, pkg)
	if err != nil || !state.Installed || state.Version != "1.2.3" {
		t.Fatalf("unexpected detection result: %+v, %v", state, err)
	}
	facts.setApps(inventory.InstalledApp{Name: "Test App Beta", Version: "9.0"})
	state, _ = Detect(ctx, facts, pkg)
	if state.Installed {
		t.Fatal("anchored pattern must not match Test App Beta")
	}
	filePkg := testPackage()
	filePkg.Detections = []Detection{{Kind: "file", Path: `%TESTDIR%\app.exe`}}
	facts = &fakeFacts{env: map[string]string{"TESTDIR": `C:\tools`}, files: map[string]FileInfo{`C:\tools\app.exe`: {Exists: true, Version: "4.5"}}}
	state, err = Detect(ctx, facts, filePkg)
	if err != nil || !state.Installed || state.Version != "4.5" {
		t.Fatalf("file detection failed: %+v, %v", state, err)
	}
	facts = &fakeFacts{appsErr: errors.New("registry unavailable")}
	if _, err := Detect(ctx, facts, pkg); err == nil {
		t.Fatal("registry failure must be reported, not treated as absent")
	}
}

func TestPlanActions(t *testing.T) {
	base := testPackage()
	base.Sources[0].Version = "2.0"
	cases := []struct {
		name      string
		installed *inventory.InstalledApp
		winget    bool
		want      Action
	}{
		{"not installed", nil, true, ActionInstall},
		{"outdated", &inventory.InstalledApp{Name: "Test App", Version: "1.0"}, true, ActionUpdate},
		{"current", &inventory.InstalledApp{Name: "Test App", Version: "2.0"}, true, ActionUpToDate},
		{"newer than catalog", &inventory.InstalledApp{Name: "Test App", Version: "2.1"}, true, ActionUpToDate},
		{"winget unavailable", nil, false, ActionUnavailable},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			facts := &fakeFacts{}
			if test.installed != nil {
				facts.setApps(*test.installed)
			}
			planner := Planner{Catalog: Catalog{Version: CatalogVersion, Packages: []Package{base}}, Facts: facts, System: testSystem(), Winget: fakeWinget{available: test.winget}}
			plan, err := planner.Build(context.Background(), nil)
			if err != nil {
				t.Fatalf("plan error: %v", err)
			}
			if len(plan.Items) != 1 {
				t.Fatalf("expected one item, got %d", len(plan.Items))
			}
			if plan.Items[0].Action != test.want {
				t.Fatalf("action = %s, want %s (reason %s)", plan.Items[0].Action, test.want, plan.Items[0].ReasonCode)
			}
		})
	}

	incompatible := base
	incompatible.Requirements = engine.OSRequirements{Versions: []string{"windows-7-sp1"}}
	facts := &fakeFacts{}
	planner := Planner{Catalog: Catalog{Version: CatalogVersion, Packages: []Package{incompatible}}, Facts: facts, System: testSystem(), Winget: fakeWinget{available: true}}
	plan, err := planner.Build(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Items[0].Action != ActionIncompatible {
		t.Fatalf("expected incompatible action, got %s", plan.Items[0].Action)
	}

	target := testPackage()
	target.Sources = []Source{
		{ID: "winget-primary", Kind: "winget", WingetID: "Test.App", Version: "2.0", Priority: 0},
		{ID: "offline-source", Kind: "file", Path: `%TESTDIR%\setup.exe`, Type: "exe", Version: "2.0", SHA256: strings.Repeat("a", 64), Priority: 5},
	}
	facts = &fakeFacts{env: map[string]string{"TESTDIR": `C:\packages`}, files: map[string]FileInfo{`C:\packages\setup.exe`: {Exists: true}}}
	planner = Planner{Catalog: Catalog{Version: CatalogVersion, Packages: []Package{target}}, Facts: facts, System: testSystem(), Winget: fakeWinget{available: false}}
	plan, err = planner.Build(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Items[0].SourceKind != "file" || plan.Items[0].Action != ActionInstall {
		t.Fatalf("offline fallback not selected: %+v", plan.Items[0])
	}
}

func TestInstallWingetSuccess(t *testing.T) {
	facts := &fakeFacts{}
	manager, runner, _ := testManager(testPackage(), facts)
	runner.results = []Execution{{ExitCode: 0}}
	runner.onRun = func(Command) {
		facts.addApp(inventory.InstalledApp{Name: "Test App", Version: "2.0"})
	}
	run, err := manager.Install(context.Background(), nil, InstallOptions{})
	if err != nil {
		t.Fatalf("install error: %v", err)
	}
	if len(run.Results) != 1 {
		t.Fatalf("expected one result, got %d", len(run.Results))
	}
	result := run.Results[0]
	if result.Status != StatusSuccess || result.InstalledAfter != "2.0" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(runner.calls) != 1 || runner.calls[0].Path != "winget" {
		t.Fatalf("unexpected runner calls: %+v", runner.calls)
	}
	args := strings.Join(runner.calls[0].Args, " ")
	if !strings.Contains(args, "install --id Test.App") || !strings.Contains(args, "--silent") {
		t.Fatalf("unexpected winget arguments: %s", args)
	}
}

func TestInstallSkipsCurrentVersion(t *testing.T) {
	facts := &fakeFacts{}
	facts.setApps(inventory.InstalledApp{Name: "Test App", Version: "2.0"})
	manager, runner, _ := testManager(testPackage(), facts)
	run, err := manager.Install(context.Background(), nil, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Results[0].Status != StatusUpToDate {
		t.Fatalf("expected up-to-date, got %s", run.Results[0].Status)
	}
	if runner.callCount() != 0 {
		t.Fatal("runner must not execute for an up-to-date package")
	}
}

func TestInstallBlockedWithoutElevation(t *testing.T) {
	pkg := testPackage()
	pkg.RequiresAdmin = true
	facts := &fakeFacts{}
	manager, runner, _ := testManager(pkg, facts)
	manager.Elevation = fakeElevation{elevated: false}
	_, err := manager.Install(context.Background(), nil, InstallOptions{})
	if !errors.Is(err, ErrElevationRequired) {
		t.Fatalf("expected elevation error, got %v", err)
	}
	if runner.callCount() != 0 {
		t.Fatal("no process may start without elevation")
	}
}

func TestInstallHashMismatch(t *testing.T) {
	pkg := testPackage()
	pkg.Sources = []Source{{
		ID: "http-source", Kind: "http", URL: "https://example.test/setup.exe", Type: "exe",
		Version: "2.0", SHA256: strings.Repeat("a", 64), Priority: 0,
	}}
	facts := &fakeFacts{}
	manager, runner, _ := testManager(pkg, facts)
	manager.Winget = fakeWinget{}
	manager.Fetcher = &fakeFetcher{data: []byte("wrong")}
	manager.CacheDir = t.TempDir()
	run, err := manager.Install(context.Background(), nil, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Results[0].Status != StatusFailed || run.Results[0].ErrorCode != CodeHashMismatch {
		t.Fatalf("unexpected result: %+v", run.Results[0])
	}
	if runner.callCount() != 0 {
		t.Fatal("installer must not run for a bad artifact")
	}
}

func TestInstallMSIRetryAndReboot(t *testing.T) {
	artifact := filepath.Join(t.TempDir(), "setup.msi")
	content := []byte("msi-bytes")
	if err := os.WriteFile(artifact, content, 0o644); err != nil {
		t.Fatal(err)
	}
	digest := HashBytes(content)
	pkg := testPackage()
	pkg.Sources = []Source{{
		ID: "local-msi", Kind: "file", Path: artifact, Type: "msi", Version: "2.0",
		SHA256: digest, Priority: 0,
	}}
	facts := &fakeFacts{}
	manager, runner, _ := testManager(pkg, facts)
	manager.Winget = fakeWinget{}
	runner.results = []Execution{{ExitCode: 1618}, {ExitCode: 3010}}
	run, err := manager.Install(context.Background(), nil, InstallOptions{RetryDelay: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	result := run.Results[0]
	if result.Status != StatusRebootRequired || !result.RebootRequired {
		t.Fatalf("expected reboot-required, got %+v", result)
	}
	if !run.RebootRequired {
		t.Fatal("run must aggregate the reboot flag")
	}
	if runner.callCount() != 2 {
		t.Fatalf("expected two installer attempts, got %d", runner.callCount())
	}
	if runner.calls[0].Path != "msiexec.exe" {
		t.Fatalf("unexpected MSI command: %+v", runner.calls[0])
	}
}

func TestInstallSignatureRequired(t *testing.T) {
	pkg := testPackage()
	pkg.Sources = []Source{{
		ID: "signed", Kind: "http", URL: "https://example.test/setup.exe", Type: "exe",
		Version: "2.0", Integrity: "signature", Signer: "Example Corp", Priority: 0,
	}}
	facts := &fakeFacts{}
	manager, runner, _ := testManager(pkg, facts)
	manager.Winget = fakeWinget{}
	manager.Fetcher = &fakeFetcher{data: []byte("unsigned")}
	manager.CacheDir = t.TempDir()
	manager.Signer = &fakeSigner{verified: false}
	run, err := manager.Install(context.Background(), nil, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Results[0].Status != StatusFailed || run.Results[0].ErrorCode != CodeSignatureFailed {
		t.Fatalf("unexpected result: %+v", run.Results[0])
	}
	if runner.callCount() != 0 {
		t.Fatal("installer must not run without a trusted signature")
	}
}

func TestInstallPortable(t *testing.T) {
	dir := t.TempDir()
	artifact := filepath.Join(dir, "PrinterOne.exe")
	content := []byte("portable-binary")
	if err := os.WriteFile(artifact, content, 0o644); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "installed")
	pkg := testPackage()
	pkg.ID = "test-portable"
	pkg.Detections = []Detection{{Kind: "file", Path: filepath.Join(target, "PrinterOne.exe")}}
	pkg.Sources = []Source{{
		ID: "portable-source", Kind: "file", Path: artifact, Type: "portable", Version: "1.0.1",
		SHA256: HashBytes(content), PortableDir: target, Priority: 0,
	}}
	facts := &fakeFacts{}
	manager, runner, _ := testManager(pkg, facts)
	manager.Winget = fakeWinget{}
	run, err := manager.Install(context.Background(), nil, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	result := run.Results[0]
	if result.Status != StatusSuccess {
		t.Fatalf("unexpected result: %+v", result)
	}
	if runner.callCount() != 0 {
		t.Fatal("portable installation must not execute the artifact")
	}
	if _, err := os.Stat(filepath.Join(target, "PrinterOne.exe")); err != nil {
		t.Fatalf("portable file missing: %v", err)
	}
}

func TestInstallUsesVerifiedCache(t *testing.T) {
	content := []byte("cached-installer")
	digest := HashBytes(content)
	cacheDir := t.TempDir()
	cached := filepath.Join(cacheDir, "sha256", digest+".exe")
	if err := os.MkdirAll(filepath.Dir(cached), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cached, content, 0o644); err != nil {
		t.Fatal(err)
	}
	pkg := testPackage()
	pkg.Sources = []Source{{
		ID: "http-source", Kind: "http", URL: "https://example.test/setup.exe", Type: "exe",
		Version: "2.0", SHA256: digest, Priority: 0,
	}}
	facts := &fakeFacts{}
	manager, runner, fetcher := testManager(pkg, facts)
	manager.Winget = fakeWinget{}
	manager.CacheDir = cacheDir
	runner.results = []Execution{{ExitCode: 0}}
	runner.onRun = func(Command) { facts.addApp(inventory.InstalledApp{Name: "Test App", Version: "2.0"}) }
	run, err := manager.Install(context.Background(), nil, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if run.Results[0].Status != StatusSuccess {
		t.Fatalf("unexpected result: %+v", run.Results[0])
	}
	if fetcher.count != 0 {
		t.Fatal("verified cached artifact must not be downloaded again")
	}
}

func TestInstallCancelledBeforeStart(t *testing.T) {
	facts := &fakeFacts{}
	manager, runner, _ := testManager(testPackage(), facts)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run, err := manager.Install(ctx, nil, InstallOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !run.Cancelled || run.Results[0].Status != StatusCancelled {
		t.Fatalf("unexpected cancellation result: %+v", run)
	}
	if runner.callCount() != 0 {
		t.Fatal("runner must not start after cancellation")
	}
}

func TestInstallerCommandDefaults(t *testing.T) {
	msi := installerCommand(Source{Type: "msi"}, `C:\cache\setup.msi`)
	if msi.Path != "msiexec.exe" || strings.Join(msi.Args, " ") != `/i C:\cache\setup.msi /qn /norestart` {
		t.Fatalf("unexpected MSI command: %+v", msi)
	}
	exe := installerCommand(Source{Type: "exe", SilentArgs: []string{"/S"}}, `C:\cache\setup.exe`)
	if exe.Path != `C:\cache\setup.exe` || strings.Join(exe.Args, " ") != "/S" {
		t.Fatalf("unexpected EXE command: %+v", exe)
	}
	if classifyExit(Source{Type: "msi"}, 1618) != exitRetry {
		t.Fatal("MSI 1618 must be retryable")
	}
	if classifyExit(Source{}, 3010) != exitReboot || classifyExit(Source{}, 1641) != exitReboot {
		t.Fatal("reboot exit codes must be recognized")
	}
}

func TestArtifactName(t *testing.T) {
	got := artifactName("pkg", Source{URL: "https://example.test/path/Firefox%20Setup.exe?x=1", Type: "exe"})
	if got != "Firefox Setup.exe" {
		t.Fatalf("unexpected artifact name: %s", got)
	}
	got = artifactName("pkg", Source{Type: "msi"})
	if got != "pkg.msi" {
		t.Fatalf("unexpected fallback name: %s", got)
	}
	if _, err := (&HTTPFetcher{}).Download(context.Background(), DownloadRequest{}); err == nil {
		t.Fatal("empty download request must fail")
	}
}

func TestCategoryOrdering(t *testing.T) {
	catalog := Catalog{Version: CatalogVersion, Packages: []Package{
		{ID: "media-one", Category: "media"},
		{ID: "browser-one", Category: "browsers"},
		{ID: "media-two", Category: "media"},
	}}
	ordered := catalog.PackagesByCategory()
	if len(ordered) != 3 || ordered[0].ID != "browser-one" || ordered[1].ID != "media-one" || ordered[2].ID != "media-two" {
		t.Fatalf("unexpected ordering: %+v", ordered)
	}
}

func ExampleCompareVersions() {
	fmt.Println(CompareVersions("1.10", "1.9"))
	// Output: 1
}
