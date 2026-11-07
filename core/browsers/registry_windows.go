//go:build windows

package browsers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"windeploy/core/shares"
)

// RegistryStore writes Chromium/Firefox enterprise policies into the Windows
// policy registry. It never modifies browser profile databases.
type RegistryStore struct {
	paths map[string]string
}

// NewRegistryStore returns the production store.
func NewRegistryStore() *RegistryStore { return &RegistryStore{} }

func newRegistryStoreWithPaths(paths map[string]string) *RegistryStore {
	return &RegistryStore{paths: paths}
}

func storeKey(browser Browser, scope Scope) string { return string(scope) + "/" + string(browser) }

func (s *RegistryStore) keyPath(browser Browser, scope Scope) (registry.Key, string) {
	if s.paths != nil {
		switch scope {
		case ScopeMachine:
			return registry.LOCAL_MACHINE, s.paths[storeKey(browser, scope)]
		default:
			return registry.CURRENT_USER, s.paths[storeKey(browser, scope)]
		}
	}
	root := registry.CURRENT_USER
	base := `Software\Policies\`
	if scope == ScopeMachine {
		root = registry.LOCAL_MACHINE
		base = `SOFTWARE\Policies\`
	}
	switch browser {
	case Chrome:
		return root, base + `Google\Chrome`
	case Edge:
		return root, base + `Microsoft\Edge`
	case Firefox:
		return root, base + `Mozilla\Firefox`
	}
	return root, ""
}

func (s *RegistryStore) open(browser Browser, scope Scope, create bool) (registry.Key, error) {
	root, path := s.keyPath(browser, scope)
	if path == "" {
		return 0, fmt.Errorf("unsupported browser policy path")
	}
	access := uint32(registry.QUERY_VALUE | registry.SET_VALUE)
	if create {
		access |= registry.CREATE_SUB_KEY
	}
	flag := uint32(0)
	if scope == ScopeMachine {
		flag = registry.WOW64_64KEY
	}
	key, err := registry.OpenKey(root, path, access|flag)
	if err != nil && create && (errors.Is(err, registry.ErrNotExist) || errors.Is(err, os.ErrNotExist)) {
		key, _, err = registry.CreateKey(root, path, access|flag)
	}
	if err != nil {
		return 0, fmt.Errorf("policy key is unavailable: %w", err)
	}
	return key, nil
}

func (s *RegistryStore) WriteString(ctx context.Context, scope Scope, browser Browser, name, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := s.open(browser, scope, true)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetStringValue(name, value); err != nil {
		return fmt.Errorf("policy value could not be written")
	}
	return nil
}

func (s *RegistryStore) WriteDWord(ctx context.Context, scope Scope, browser Browser, name string, value uint32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := s.open(browser, scope, true)
	if err != nil {
		return err
	}
	defer key.Close()
	if err := key.SetDWordValue(name, value); err != nil {
		return fmt.Errorf("policy value could not be written")
	}
	return nil
}

func (s *RegistryStore) DeleteValue(ctx context.Context, scope Scope, browser Browser, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := s.open(browser, scope, false)
	if err != nil {
		return nil // Missing key means the policy is already absent.
	}
	defer key.Close()
	if err := key.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return fmt.Errorf("policy value could not be removed")
	}
	return nil
}

// WindowsLocator finds installed browser executables without running them.
type WindowsLocator struct{}

// Executable implements Locator through App Paths and standard install folders.
func (WindowsLocator) Executable(ctx context.Context, browser Browser) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	names := map[Browser]string{Chrome: "chrome.exe", Edge: "msedge.exe", Firefox: "firefox.exe"}
	name, ok := names[browser]
	if !ok {
		return "", fmt.Errorf("unsupported browser")
	}
	for _, root := range []registry.Key{registry.LOCAL_MACHINE, registry.CURRENT_USER} {
		path := `SOFTWARE\Microsoft\Windows\CurrentVersion\App Paths\` + name
		access := uint32(registry.QUERY_VALUE)
		if root == registry.LOCAL_MACHINE {
			access |= registry.WOW64_64KEY
		}
		key, err := registry.OpenKey(root, path, access)
		if err != nil {
			continue
		}
		value, _, valueErr := key.GetStringValue("")
		key.Close()
		if valueErr == nil && value != "" {
			if info, statErr := os.Stat(value); statErr == nil && info.Mode().IsRegular() {
				return value, nil
			}
		}
	}
	candidates := []string{}
	switch browser {
	case Chrome:
		candidates = append(candidates,
			filepath.Join(os.Getenv("ProgramFiles"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), `Google\Chrome\Application\chrome.exe`),
			filepath.Join(os.Getenv("LOCALAPPDATA"), `Google\Chrome\Application\chrome.exe`))
	case Edge:
		candidates = append(candidates,
			filepath.Join(os.Getenv("ProgramFiles(x86)"), `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(os.Getenv("ProgramFiles"), `Microsoft\Edge\Application\msedge.exe`))
	case Firefox:
		candidates = append(candidates,
			filepath.Join(os.Getenv("ProgramFiles"), `Mozilla Firefox\firefox.exe`),
			filepath.Join(os.Getenv("ProgramFiles(x86)"), `Mozilla Firefox\firefox.exe`))
	}
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("browser executable was not found")
}

// WindowsShortcut adapts the native .lnk creator for browser shortcuts.
type WindowsShortcut struct{}

func (WindowsShortcut) Create(ctx context.Context, path, target, arguments, description string) error {
	spec := shares.ShortcutSpec{Path: path, Target: target, Arguments: arguments, Description: description}
	adapter := shares.WindowsShortcut{}
	if err := adapter.Create(ctx, spec); err != nil {
		return err
	}
	if matches, err := adapter.Matches(ctx, spec); err != nil || !matches {
		return ErrWebAppsRequired
	}
	return nil
}

// DesktopDirectory returns the real desktop folder.
func DesktopDirectory() (string, error) { return shares.DesktopDirectory() }

// WindowsElevation reports the current token elevation.
type WindowsElevation struct{}

func (WindowsElevation) Elevated(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false, err
	}
	defer token.Close()
	return token.IsElevated(), nil
}
