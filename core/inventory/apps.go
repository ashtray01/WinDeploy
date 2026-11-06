package inventory

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// InstalledApp is a visible Uninstall registry entry, not proof of an executable's
// presence. It excludes uninstall commands, install paths and credentials.
type InstalledApp struct {
	Key              string   `json:"key"`
	Scope            string   `json:"scope"`
	Views            []string `json:"views"`
	Name             string   `json:"name"`
	Version          string   `json:"version,omitempty"`
	Publisher        string   `json:"publisher,omitempty"`
	WindowsInstaller bool     `json:"windows_installer"`
}
type AppProvider interface {
	Apps(context.Context) ([]InstalledApp, error)
}
type appEntry struct {
	Key, Name, Version, Publisher string
	Hidden, WindowsInstaller      bool
}
type appReader func(context.Context, string, string) ([]appEntry, error)

func collectApps(ctx context.Context, arch string, read appReader) ([]InstalledApp, error) {
	views := []string{"32"}
	if arch == "amd64" || arch == "arm64" {
		views = append(views, "64")
	} else if arch != "386" {
		return nil, fmt.Errorf("native architecture unknown for registry inventory")
	}
	result := []InstalledApp{}
	positions := map[string]int{}
	failed := false
	for _, scope := range []string{"machine", "current-user"} {
		for _, view := range views {
			if err := ctx.Err(); err != nil {
				return result, err
			}
			entries, err := read(ctx, scope, view)
			if err != nil {
				failed = true
			}
			if err := ctx.Err(); err != nil {
				return result, err
			}
			for _, entry := range entries {
				name := strings.TrimSpace(entry.Name)
				if entry.Hidden || name == "" {
					continue
				}
				version, publisher := strings.TrimSpace(entry.Version), strings.TrimSpace(entry.Publisher)
				// Shared HKCU views can yield identical entries. Preserve distinct
				// keys, versions, scopes and installer flags instead of merging by name.
				identity := fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%s\x00%t", scope, strings.ToLower(entry.Key), name, version, publisher, entry.WindowsInstaller)
				if index, exists := positions[identity]; exists {
					result[index].Views = append(result[index].Views, view)
					continue
				}
				positions[identity] = len(result)
				result = append(result, InstalledApp{Key: entry.Key, Scope: scope, Views: []string{view}, Name: name, Version: version, Publisher: publisher, WindowsInstaller: entry.WindowsInstaller})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		left, right := strings.ToLower(a.Name)+"\x00"+a.Version+"\x00"+a.Scope+"\x00"+a.Key, strings.ToLower(b.Name)+"\x00"+b.Version+"\x00"+b.Scope+"\x00"+b.Key
		if left == right {
			return a.Views[0] < b.Views[0]
		}
		return left < right
	})
	if failed {
		return result, fmt.Errorf("some uninstall registry entries are unavailable")
	}
	return result, nil
}
