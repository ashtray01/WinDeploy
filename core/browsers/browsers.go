// Package browsers applies supported enterprise browser policies without editing
// existing user profiles. It never copies or rewrites profile databases and never
// installs extensions.
package browsers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

// Browser identifies a supported policy target.
type Browser string

const (
	Chrome  Browser = "chrome"
	Edge    Browser = "edge"
	Firefox Browser = "firefox"
)

// Scope selects the current user or the machine policy hive.
type Scope string

const (
	ScopeUser    Scope = "user"
	ScopeMachine Scope = "machine"
)

// Bookmark is one managed bookmark.
type Bookmark struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// WebApp is a desktop shortcut that opens a URL in an app window.
type WebApp struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Policy is the requested browser configuration. Empty fields are not written.
type Policy struct {
	Browser     Browser    `json:"browser"`
	Scope       Scope      `json:"scope"`
	Homepage    string     `json:"homepage,omitempty"`
	StartupURLs []string   `json:"startup_urls,omitempty"`
	Bookmarks   []Bookmark `json:"bookmarks,omitempty"`
	WebApps     []WebApp   `json:"webapps,omitempty"`
}

// Change describes one planned or applied policy value.
type Change struct {
	Browser Browser `json:"browser"`
	Scope   Scope   `json:"scope"`
	Name    string  `json:"name"`
	Action  string  `json:"action"`
}

// Result is a safe apply outcome.
type Result struct {
	Applied   []Change `json:"applied"`
	Shortcuts []string `json:"shortcuts"`
	Warnings  []string `json:"warnings"`
	DryRun    bool     `json:"dry_run"`
}

// ValueStore writes policy registry values. Implementations must not touch
// browser profile files.
type ValueStore interface {
	WriteString(ctx context.Context, scope Scope, browser Browser, name, value string) error
	WriteDWord(ctx context.Context, scope Scope, browser Browser, name string, value uint32) error
	DeleteValue(ctx context.Context, scope Scope, browser Browser, name string) error
}

// ShortcutCreator creates desktop .lnk files.
type ShortcutCreator interface {
	Create(ctx context.Context, path, target, arguments, description string) error
}

// Locator finds the installed browser executable.
type Locator interface {
	Executable(ctx context.Context, browser Browser) (string, error)
}

// Elevation reports administrator rights for machine-scope policies.
type Elevation interface {
	Elevated(ctx context.Context) (bool, error)
}

// ErrElevationRequired is returned for machine scope without elevation.
var ErrElevationRequired = errors.New("administrator privileges are required for machine policy")

// Manager applies validated browser policies.
type Manager struct {
	Store     ValueStore
	Shortcuts ShortcutCreator
	Locator   Locator
	Elevation Elevation
	// DesktopDir is the real desktop folder; empty disables web-app shortcuts.
	DesktopDir string
}

// Validate checks policy structure and URL safety.
func Validate(policy Policy) error {
	switch policy.Browser {
	case Chrome, Edge, Firefox:
	default:
		return fmt.Errorf("unsupported browser")
	}
	switch policy.Scope {
	case ScopeUser, ScopeMachine:
	default:
		return fmt.Errorf("unsupported policy scope")
	}
	if len(policy.StartupURLs) > 50 || len(policy.Bookmarks) > 200 || len(policy.WebApps) > 50 {
		return fmt.Errorf("policy exceeds item limits")
	}
	validateURL := func(value string) error {
		parsed, err := url.Parse(strings.TrimSpace(value))
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
			return fmt.Errorf("policy requires plain http(s) URLs without credentials")
		}
		return nil
	}
	if policy.Homepage != "" {
		if err := validateURL(policy.Homepage); err != nil {
			return err
		}
	}
	for _, raw := range policy.StartupURLs {
		if err := validateURL(raw); err != nil {
			return err
		}
	}
	for _, bookmark := range policy.Bookmarks {
		if strings.TrimSpace(bookmark.Name) == "" || len(bookmark.Name) > 200 || strings.ContainsAny(bookmark.Name, "\x00\r\n") {
			return fmt.Errorf("bookmark name is invalid")
		}
		if err := validateURL(bookmark.URL); err != nil {
			return err
		}
	}
	if len(policy.WebApps) > 0 && policy.Browser == Firefox {
		return fmt.Errorf("firefox does not support app-window shortcuts in this implementation")
	}
	for _, app := range policy.WebApps {
		if strings.TrimSpace(app.Name) == "" || len(app.Name) > 120 || strings.ContainsAny(app.Name, "\x00\r\n") {
			return fmt.Errorf("web app name is invalid")
		}
		if err := validateURL(app.URL); err != nil {
			return err
		}
	}
	return nil
}

// Apply writes only supported registry policies and optional desktop shortcuts.
func (m *Manager) Apply(ctx context.Context, policy Policy, dryRun bool) (Result, error) {
	result := Result{Applied: []Change{}, Shortcuts: []string{}, Warnings: []string{}, DryRun: dryRun}
	if err := Validate(policy); err != nil {
		return result, err
	}
	if m.Store == nil {
		return result, fmt.Errorf("policy store is required")
	}
	if policy.Scope == ScopeMachine && !dryRun {
		if m.Elevation == nil {
			return result, ErrElevationRequired
		}
		elevated, err := m.Elevation.Elevated(ctx)
		if err != nil || !elevated {
			return result, ErrElevationRequired
		}
	}
	writeString := func(name, value string) error {
		result.Applied = append(result.Applied, Change{Browser: policy.Browser, Scope: policy.Scope, Name: name, Action: actionFor(dryRun)})
		if dryRun {
			return nil
		}
		return m.Store.WriteString(ctx, policy.Scope, policy.Browser, name, value)
	}
	if policy.Browser == Chrome || policy.Browser == Edge {
		if policy.Homepage != "" {
			if err := writeString("HomepageLocation", policy.Homepage); err != nil {
				return result, fmt.Errorf("homepage policy could not be written")
			}
		}
		if len(policy.StartupURLs) > 0 {
			result.Applied = append(result.Applied, Change{Browser: policy.Browser, Scope: policy.Scope, Name: "RestoreOnStartup", Action: actionFor(dryRun)})
			result.Applied = append(result.Applied, Change{Browser: policy.Browser, Scope: policy.Scope, Name: "RestoreOnStartupURLs", Action: actionFor(dryRun)})
			if !dryRun {
				if err := m.Store.WriteDWord(ctx, policy.Scope, policy.Browser, "RestoreOnStartup", 4); err != nil {
					return result, fmt.Errorf("startup policy could not be written")
				}
				if err := m.Store.WriteString(ctx, policy.Scope, policy.Browser, "RestoreOnStartupURLs", strings.Join(policy.StartupURLs, "\n")); err != nil {
					return result, fmt.Errorf("startup URLs policy could not be written")
				}
			}
		}
		if len(policy.Bookmarks) > 0 {
			payload, err := chromiumBookmarks(policy.Bookmarks)
			if err != nil {
				return result, err
			}
			if err := writeString("ManagedBookmarks", payload); err != nil {
				return result, fmt.Errorf("managed bookmarks policy could not be written")
			}
		}
	} else {
		if policy.Homepage != "" {
			payload, _ := json.Marshal(map[string]string{"URL": policy.Homepage, "StartPage": "homepage"})
			if err := writeString("Homepage", string(payload)); err != nil {
				return result, fmt.Errorf("firefox homepage policy could not be written")
			}
		}
		if len(policy.StartupURLs) > 0 {
			result.Warnings = append(result.Warnings, "Firefox не поддерживает список стартовых URL в этой реализации; задана домашняя страница.")
		}
		if len(policy.Bookmarks) > 0 {
			payload, err := firefoxBookmarks(policy.Bookmarks)
			if err != nil {
				return result, err
			}
			if err := writeString("Bookmarks", payload); err != nil {
				return result, fmt.Errorf("firefox bookmarks policy could not be written")
			}
		}
	}
	if len(policy.WebApps) > 0 {
		if err := m.createWebApps(ctx, policy, &result); err != nil {
			return result, err
		}
	}
	return result, nil
}

func (m *Manager) createWebApps(ctx context.Context, policy Policy, result *Result) error {
	if m.Locator == nil || m.Shortcuts == nil || m.DesktopDir == "" {
		result.Warnings = append(result.Warnings, "Ярлыки веб-приложений не созданы: компонент недоступен.")
		return nil
	}
	if policy.Browser != Chrome && policy.Browser != Edge {
		result.Warnings = append(result.Warnings, "Веб-приложения поддерживаются только для Chrome и Edge.")
		return nil
	}
	executable, err := m.Locator.Executable(ctx, policy.Browser)
	if err != nil || executable == "" {
		result.Warnings = append(result.Warnings, "Браузер не найден; ярлыки веб-приложений не созданы.")
		return nil
	}
	for _, app := range policy.WebApps {
		path := filepath.Join(m.DesktopDir, sanitizeName(app.Name)+".lnk")
		if result.DryRun {
			result.Shortcuts = append(result.Shortcuts, path)
			continue
		}
		if err := m.Shortcuts.Create(ctx, path, executable, "--app="+app.URL, app.Name); err != nil {
			result.Warnings = append(result.Warnings, "Не удалось создать ярлык: "+app.Name)
			continue
		}
		result.Shortcuts = append(result.Shortcuts, path)
	}
	return nil
}

func chromiumBookmarks(bookmarks []Bookmark) (string, error) {
	items := []any{map[string]string{"toplevel_name": "Корпоративные"}}
	for _, bookmark := range bookmarks {
		items = append(items, map[string]string{"name": bookmark.Name, "url": bookmark.URL})
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("bookmarks could not be encoded")
	}
	return string(payload), nil
}

func firefoxBookmarks(bookmarks []Bookmark) (string, error) {
	items := make([]map[string]string, 0, len(bookmarks))
	for _, bookmark := range bookmarks {
		items = append(items, map[string]string{"Title": bookmark.Name, "URL": bookmark.URL, "Placement": "toolbar"})
	}
	payload, err := json.Marshal(items)
	if err != nil {
		return "", fmt.Errorf("bookmarks could not be encoded")
	}
	return string(payload), nil
}

func actionFor(dryRun bool) string {
	if dryRun {
		return "planned"
	}
	return "applied"
}

func sanitizeName(value string) string {
	safe := make([]rune, 0, len(value))
	for _, r := range strings.TrimSpace(value) {
		switch {
		case r < 32, strings.ContainsRune(`<>:"/\|?*`, r):
			safe = append(safe, '_')
		default:
			safe = append(safe, r)
		}
	}
	name := strings.TrimSpace(string(safe))
	if name == "" {
		name = "web-app"
	}
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	return name
}
