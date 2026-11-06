//go:build windows

package browsers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

func TestRegistryStoreRoundTrip(t *testing.T) {
	base := `Software\WinDeployTests\BrowserPolicy` + stringsForTest(t)
	paths := map[string]string{
		"user/chrome":     base + `\Chrome`,
		"user/edge":       base + `\Edge`,
		"user/firefox":    base + `\Firefox`,
		"machine/chrome":  base + `\ChromeMachine`,
		"machine/edge":    base + `\EdgeMachine`,
		"machine/firefox": base + `\FirefoxMachine`,
	}
	store := newRegistryStoreWithPaths(paths)
	ctx := context.Background()
	if err := store.WriteString(ctx, ScopeUser, Chrome, "HomepageLocation", "https://intranet.example/"); err != nil {
		if errors.Is(err, windows.ERROR_ACCESS_DENIED) {
			t.Skip("registry writes are denied by the current low-integrity sandbox")
		}
		t.Fatalf("write string failed: %v", err)
	}
	if err := store.WriteDWord(ctx, ScopeUser, Chrome, "RestoreOnStartup", 4); err != nil {
		t.Fatalf("write dword failed: %v", err)
	}
	key, err := registry.OpenKey(registry.CURRENT_USER, paths["user/chrome"], registry.QUERY_VALUE)
	if err != nil {
		t.Fatalf("test key missing: %v", err)
	}
	value, _, _ := key.GetStringValue("HomepageLocation")
	dword, _, _ := key.GetIntegerValue("RestoreOnStartup")
	key.Close()
	if value != "https://intranet.example/" || dword != 4 {
		t.Fatalf("unexpected stored policy: %q %d", value, dword)
	}
	if err := store.DeleteValue(ctx, ScopeUser, Chrome, "HomepageLocation"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	for _, path := range paths {
		registry.DeleteKey(registry.CURRENT_USER, path)
	}
	registry.DeleteKey(registry.CURRENT_USER, base)
}

func stringsForTest(t *testing.T) string {
	t.Helper()
	return `\` + filepath.Base(t.TempDir())
}

func TestWindowsLocatorFindsInstalledBrowser(t *testing.T) {
	ctx := context.Background()
	found := false
	for _, browser := range []Browser{Chrome, Edge, Firefox} {
		path, err := (WindowsLocator{}).Executable(ctx, browser)
		if err != nil {
			continue
		}
		found = true
		if _, statErr := os.Stat(path); statErr != nil {
			t.Fatalf("locator returned a missing executable: %v", statErr)
		}
	}
	if !found {
		t.Skip("no supported browser is installed for the locator test")
	}
}

func TestDesktopDirectory(t *testing.T) {
	dir, err := DesktopDirectory()
	if err != nil {
		t.Fatalf("desktop directory failed: %v", err)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Fatalf("desktop directory is inaccessible: %v", err)
	}
}
