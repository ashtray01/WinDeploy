//go:build windows

package inventory

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/sys/windows/registry"
)

const uninstallKey = `SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`

func readAppEntries(ctx context.Context, scope, view string) ([]appEntry, error) {
	root := registry.LOCAL_MACHINE
	if scope == "current-user" {
		root = registry.CURRENT_USER
	}
	flag := uint32(registry.WOW64_32KEY)
	if view == "64" {
		flag = registry.WOW64_64KEY
	}
	key, err := registry.OpenKey(root, uninstallKey, registry.QUERY_VALUE|registry.ENUMERATE_SUB_KEYS|flag)
	if errors.Is(err, registry.ErrNotExist) {
		return []appEntry{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open uninstall registry failed")
	}
	defer key.Close()
	names, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return nil, fmt.Errorf("enumerate uninstall registry failed")
	}
	if len(names) > 100000 {
		return nil, fmt.Errorf("uninstall registry exceeds limit")
	}
	entries := make([]appEntry, 0, len(names))
	failed := false
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return entries, err
		}
		child, err := registry.OpenKey(key, name, registry.QUERY_VALUE|flag)
		if errors.Is(err, registry.ErrNotExist) {
			continue
		}
		if err != nil {
			failed = true
			continue
		}
		entry, valid, err := readAppEntry(child, name)
		child.Close()
		if err != nil {
			failed = true
		}
		if valid {
			entries = append(entries, entry)
		}
	}
	if failed {
		return entries, fmt.Errorf("some uninstall registry entries are unavailable")
	}
	return entries, nil
}

func readAppEntry(key registry.Key, name string) (appEntry, bool, error) {
	display, _, err := key.GetStringValue("DisplayName")
	if errors.Is(err, registry.ErrNotExist) {
		return appEntry{}, false, nil
	}
	if err != nil {
		return appEntry{}, false, fmt.Errorf("read app name failed")
	}
	entry := appEntry{Key: name, Name: display}
	failed := false
	entry.Version, _, err = key.GetStringValue("DisplayVersion")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		failed = true
	}
	entry.Publisher, _, err = key.GetStringValue("Publisher")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		failed = true
	}
	hidden, _, err := key.GetIntegerValue("SystemComponent")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		return appEntry{}, false, fmt.Errorf("read app visibility failed")
	}
	entry.Hidden = hidden != 0
	installer, _, err := key.GetIntegerValue("WindowsInstaller")
	if err != nil && !errors.Is(err, registry.ErrNotExist) {
		failed = true
	}
	entry.WindowsInstaller = installer == 1
	if failed {
		return entry, true, fmt.Errorf("read optional app metadata failed")
	}
	return entry, true, nil
}

// Apps enumerates HKLM and the effective process user's HKCU. It never queries
// Win32_Product (which can trigger MSI repairs), executes commands or impersonates
// the interactive user. AppX/MSIX and arbitrary portable software are not covered.
func (n Native) Apps(ctx context.Context) ([]InstalledApp, error) {
	system, err := n.System(ctx)
	if err != nil {
		return nil, err
	}
	return collectApps(ctx, system.Architecture, readAppEntries)
}
