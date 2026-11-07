package browsers

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type fakeStore struct {
	strings map[string]string
	dwords  map[string]uint32
	writes  int
}

func (s *fakeStore) key(scope Scope, browser Browser, name string) string {
	return string(scope) + "/" + string(browser) + "/" + name
}

func (s *fakeStore) WriteString(_ context.Context, scope Scope, browser Browser, name, value string) error {
	s.writes++
	if s.strings == nil {
		s.strings = map[string]string{}
	}
	s.strings[s.key(scope, browser, name)] = value
	return nil
}

func (s *fakeStore) WriteDWord(_ context.Context, scope Scope, browser Browser, name string, value uint32) error {
	s.writes++
	if s.dwords == nil {
		s.dwords = map[string]uint32{}
	}
	s.dwords[s.key(scope, browser, name)] = value
	return nil
}

func (s *fakeStore) DeleteValue(context.Context, Scope, Browser, string) error { return nil }

func (s *fakeStore) ReadString(_ context.Context, scope Scope, browser Browser, name string) (string, bool, error) {
	value, ok := s.strings[s.key(scope, browser, name)]
	return value, ok, nil
}

func (s *fakeStore) ReadDWord(_ context.Context, scope Scope, browser Browser, name string) (uint32, bool, error) {
	value, ok := s.dwords[s.key(scope, browser, name)]
	return value, ok, nil
}

// tamperingStore accepts writes but reads back a different value, like a policy
// overwritten by another management tool.
type tamperingStore struct {
	*fakeStore
	readErr error
}

func (s tamperingStore) ReadString(context.Context, Scope, Browser, string) (string, bool, error) {
	if s.readErr != nil {
		return "", false, s.readErr
	}
	return "different", true, nil
}

type fakeShortcut struct {
	paths []string
	args  []string
	err   error
}

func (s *fakeShortcut) Create(_ context.Context, path, _ string, arguments, _ string) error {
	s.paths = append(s.paths, path)
	s.args = append(s.args, arguments)
	return s.err
}

type fakeLocator struct{ path string }

func (l fakeLocator) Executable(context.Context, Browser) (string, error) {
	if l.path == "" {
		return "", errors.New("not found")
	}
	return l.path, nil
}

type fakeElevation bool

func (e fakeElevation) Elevated(context.Context) (bool, error) { return bool(e), nil }

func chromePolicy() Policy {
	return Policy{
		Browser: Chrome, Scope: ScopeUser, Homepage: "https://intranet.example/",
		StartupURLs: []string{"https://intranet.example/", "https://mail.example/"},
		Bookmarks:   []Bookmark{{Name: "Портал", URL: "https://intranet.example/"}},
	}
}

func TestValidatePolicy(t *testing.T) {
	if err := Validate(chromePolicy()); err != nil {
		t.Fatalf("valid policy rejected: %v", err)
	}
	bad := chromePolicy()
	bad.Homepage = "http://user:pass@example.test/"
	if err := Validate(bad); err == nil {
		t.Fatal("credentials in URL accepted")
	}
	bad = chromePolicy()
	bad.Browser = "opera"
	if err := Validate(bad); err == nil {
		t.Fatal("unsupported browser accepted")
	}
	bad = Policy{Browser: Firefox, Scope: ScopeUser, WebApps: []WebApp{{Name: "App", URL: "https://example.test/"}}}
	if err := Validate(bad); err == nil {
		t.Fatal("firefox web app accepted")
	}
}

func TestApplyChromiumPolicies(t *testing.T) {
	store := &fakeStore{}
	manager := &Manager{Store: store, Elevation: fakeElevation(true)}
	result, err := manager.Apply(context.Background(), chromePolicy(), false)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if store.strings["user/chrome/HomepageLocation"] != "https://intranet.example/" {
		t.Fatalf("homepage not written: %+v", store.strings)
	}
	if store.dwords["user/chrome/RestoreOnStartup"] != 4 {
		t.Fatalf("startup mode not written: %+v", store.dwords)
	}
	if store.strings["user/chrome/RestoreOnStartupURLs"] != "https://intranet.example/\nhttps://mail.example/" {
		t.Fatalf("startup URLs not written: %q", store.strings["user/chrome/RestoreOnStartupURLs"])
	}
	var bookmarks []any
	if err := json.Unmarshal([]byte(store.strings["user/chrome/ManagedBookmarks"]), &bookmarks); err != nil || len(bookmarks) != 2 {
		t.Fatalf("invalid managed bookmarks: %q, %v", store.strings["user/chrome/ManagedBookmarks"], err)
	}
	if len(result.Applied) != 4 {
		t.Fatalf("unexpected changes: %+v", result.Applied)
	}
}

func TestApplyChromiumConfirmsEveryWrittenPolicy(t *testing.T) {
	store := &fakeStore{}
	manager := &Manager{Store: store, Elevation: fakeElevation(true)}
	result, err := manager.Apply(context.Background(), chromePolicy(), false)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("confirmed policies produced warnings: %+v", result.Warnings)
	}
}

func TestApplyReportsUnconfirmedPolicyValues(t *testing.T) {
	for _, test := range []struct {
		name  string
		store ValueStore
	}{
		{"different value", tamperingStore{fakeStore: &fakeStore{}}},
		{"read error", tamperingStore{fakeStore: &fakeStore{}, readErr: errors.New("denied")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			manager := &Manager{Store: test.store, Elevation: fakeElevation(true)}
			result, err := manager.Apply(context.Background(), chromePolicy(), false)
			if err != nil {
				t.Fatalf("apply failed: %v", err)
			}
			if len(result.Warnings) == 0 {
				t.Fatal("unconfirmed policy values reported as confirmed")
			}
			joined := strings.Join(result.Warnings, " ")
			if !strings.Contains(joined, "не подтверждена") {
				t.Fatalf("unexpected warnings: %+v", result.Warnings)
			}
		})
	}
}

// writeOnlyStore implements ValueStore without ValueReader.
type writeOnlyStore struct {
	strings map[string]string
	dwords  map[string]uint32
}

func (s *writeOnlyStore) WriteString(_ context.Context, _ Scope, _ Browser, name, value string) error {
	if s.strings == nil {
		s.strings = map[string]string{}
	}
	s.strings[name] = value
	return nil
}
func (s *writeOnlyStore) WriteDWord(_ context.Context, _ Scope, _ Browser, name string, value uint32) error {
	if s.dwords == nil {
		s.dwords = map[string]uint32{}
	}
	s.dwords[name] = value
	return nil
}
func (s *writeOnlyStore) DeleteValue(context.Context, Scope, Browser, string) error { return nil }

func TestApplyWithoutReaderIsNotSilentlySuccessful(t *testing.T) {
	manager := &Manager{Store: &writeOnlyStore{}}
	result, err := manager.Apply(context.Background(), chromePolicy(), false)
	if err != nil {
		t.Fatalf("apply failed: %v", err)
	}
	if len(result.Warnings) == 0 {
		t.Fatal("store without read-back reported unconditional success")
	}
}

func TestApplyDryRunWritesNothing(t *testing.T) {
	store := &fakeStore{}
	manager := &Manager{Store: store}
	result, err := manager.Apply(context.Background(), chromePolicy(), true)
	if err != nil {
		t.Fatal(err)
	}
	if store.writes != 0 || !result.DryRun || len(result.Applied) == 0 {
		t.Fatalf("dry-run changed state: %+v", result)
	}
}

func TestMachineScopeRequiresElevation(t *testing.T) {
	policy := chromePolicy()
	policy.Scope = ScopeMachine
	manager := &Manager{Store: &fakeStore{}, Elevation: fakeElevation(false)}
	if _, err := manager.Apply(context.Background(), policy, false); !errors.Is(err, ErrElevationRequired) {
		t.Fatalf("expected elevation error, got %v", err)
	}
	manager.Elevation = fakeElevation(true)
	if _, err := manager.Apply(context.Background(), policy, false); err != nil {
		t.Fatalf("elevated machine policy failed: %v", err)
	}
}

func TestApplyFirefoxHomepageAndBookmarks(t *testing.T) {
	store := &fakeStore{}
	manager := &Manager{Store: store}
	policy := Policy{Browser: Firefox, Scope: ScopeUser, Homepage: "https://intranet.example/", StartupURLs: []string{"https://intranet.example/"}, Bookmarks: []Bookmark{{Name: "Портал", URL: "https://intranet.example/"}}}
	result, err := manager.Apply(context.Background(), policy, false)
	if err != nil {
		t.Fatal(err)
	}
	var homepage map[string]string
	if err := json.Unmarshal([]byte(store.strings["user/firefox/Homepage"]), &homepage); err != nil || homepage["URL"] != "https://intranet.example/" {
		t.Fatalf("invalid firefox homepage: %q", store.strings["user/firefox/Homepage"])
	}
	if !strings.Contains(store.strings["user/firefox/Bookmarks"], "Портал") {
		t.Fatalf("firefox bookmarks missing: %q", store.strings["user/firefox/Bookmarks"])
	}
	if len(result.Warnings) == 0 {
		t.Fatal("firefox startup limitation must produce a warning")
	}
}

func TestWebAppShortcuts(t *testing.T) {
	store := &fakeStore{}
	shortcuts := &fakeShortcut{}
	manager := &Manager{Store: store, Locator: fakeLocator{path: `C:\Program Files\Google\Chrome\Application\chrome.exe`}, Shortcuts: shortcuts, DesktopDir: `C:\Users\Test\Desktop`}
	policy := chromePolicy()
	policy.WebApps = []WebApp{{Name: "Портал", URL: "https://intranet.example/"}}
	result, err := manager.Apply(context.Background(), policy, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(shortcuts.paths) != 1 || !strings.HasSuffix(shortcuts.paths[0], "Портал.lnk") {
		t.Fatalf("shortcut not created: %+v", shortcuts.paths)
	}
	if shortcuts.args[0] != "--app=https://intranet.example/" {
		t.Fatalf("unexpected shortcut arguments: %q", shortcuts.args[0])
	}
	if len(result.Shortcuts) != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRequiredShortcutMissingBrowserFailsBeforePolicyWrites(t *testing.T) {
	manager := &Manager{Store: &fakeStore{}, Locator: fakeLocator{}, Shortcuts: &fakeShortcut{}, DesktopDir: `C:\Desktop`}
	policy := chromePolicy()
	policy.WebApps = []WebApp{{Name: "App", URL: "https://example.test/"}}
	result, err := manager.Apply(context.Background(), policy, false)
	if !errors.Is(err, ErrWebAppsRequired) {
		t.Fatalf("missing required browser accepted: %v", err)
	}
	if manager.Store.(*fakeStore).writes != 0 {
		t.Fatal("known missing browser caused partial policy writes")
	}
	if len(result.Warnings) == 0 {
		t.Fatal("missing browser must be reported as a warning")
	}
}

func TestRequiredWebAppCreationFailureIsNotSuccess(t *testing.T) {
	policy := chromePolicy()
	policy.WebApps = []WebApp{{Name: "App", URL: "https://example.test/"}}
	manager := &Manager{Store: &fakeStore{}, Locator: fakeLocator{path: `C:\Browser\browser.exe`}, Shortcuts: &fakeShortcut{err: errors.New("denied")}, DesktopDir: `C:\Desktop`}
	result, err := manager.Apply(context.Background(), policy, false)
	if !errors.Is(err, ErrWebAppsRequired) || len(result.Shortcuts) != 0 || len(result.Warnings) == 0 {
		t.Fatalf("false webapp success: %+v %v", result, err)
	}
}
