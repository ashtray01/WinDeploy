// Package shares connects UNC resources and creates desktop shortcuts without
// storing plaintext passwords in profiles, logs or command lines.
package shares

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

var shareID = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,63}$`)
var uncPattern = regexp.MustCompile(`^\\\\[A-Za-z0-9._-]+\\[^\\/:*?"<>|]+$`)

// Share describes one UNC connection. Password is never part of the model.
type Share struct {
	ID         string `yaml:"id" json:"id"`
	UNC        string `yaml:"unc" json:"unc"`
	Name       string `yaml:"name,omitempty" json:"name,omitempty"`
	Username   string `yaml:"username,omitempty" json:"username,omitempty"`
	Persistent bool   `yaml:"persistent,omitempty" json:"persistent"`
	Shortcut   bool   `yaml:"shortcut,omitempty" json:"shortcut"`
}

// ErrUnsupported is returned by native adapters outside Windows.
var ErrUnsupported = errors.New("share operations are supported on Windows only")

// Validate checks a share definition without touching the system.
func (s Share) Validate() error {
	if !shareID.MatchString(s.ID) {
		return fmt.Errorf("invalid share ID")
	}
	if !uncPattern.MatchString(strings.TrimRight(s.UNC, `\`)) {
		return fmt.Errorf("share requires a plain \\\\server\\share UNC path")
	}
	if strings.ContainsAny(s.Username, "\x00\r\n") || len(s.Username) > 256 {
		return fmt.Errorf("invalid user name")
	}
	if strings.Contains(s.UNC, "@") {
		return fmt.Errorf("credentials must not be embedded in the UNC path")
	}
	return nil
}

// Server extracts the server component from a validated UNC path.
func (s Share) Server() string {
	trimmed := strings.TrimPrefix(strings.TrimRight(s.UNC, `\`), `\\`)
	if index := strings.Index(trimmed, `\`); index >= 0 {
		return trimmed[:index]
	}
	return trimmed
}

// CredentialTarget is the Windows Credential Manager target for one server.
// A generic credential is used because Windows rejects arbitrary blob secrets for
// domain-password entries outside a domain; the target still isolates servers.
func CredentialTarget(server string) string {
	return "WinDeploy:share:" + strings.ToLower(server)
}

// Credential is a username/password pair held only in memory. The password is
// never serialized to a report or profile.
type Credential struct {
	Username string
	Password string
}

// CredentialStore persists per-server credentials in the Windows Credential
// Manager (or an injected equivalent in tests).
type CredentialStore interface {
	Save(ctx context.Context, target, username, password string) error
	Read(ctx context.Context, target string) (Credential, bool, error)
	Delete(ctx context.Context, target string) error
}

// ConnectionRequest is one WNet connection request.
type ConnectionRequest struct {
	RemoteName string
	Username   string
	Password   string
	Persistent bool
}

// Connection is an existing mapped or UNC connection.
type Connection struct {
	RemoteName string
	LocalName  string
}

// Connector adds, lists and removes SMB connections through Win32 WNet APIs.
type Connector interface {
	Connect(ctx context.Context, request ConnectionRequest) error
	List(ctx context.Context) ([]Connection, error)
	Disconnect(ctx context.Context, remoteName string, force bool) error
}

// ShortcutSpec describes a desktop shortcut.
type ShortcutSpec struct {
	Path        string
	Target      string
	Arguments   string
	Description string
}

// ShortcutCreator creates .lnk files.
type ShortcutCreator interface {
	Create(ctx context.Context, spec ShortcutSpec) error
}

// ErrCredentialConflict wraps Windows Error 1219: an existing connection to the
// same server uses different credentials.
var ErrCredentialConflict = errors.New("server already has a connection with different credentials")

// ConflictError carries safe details and the connections that occupy the server.
type ConflictError struct {
	Server   string
	Existing []string
}

func (e *ConflictError) Error() string { return ErrCredentialConflict.Error() }
func (e *ConflictError) Unwrap() error { return ErrCredentialConflict }

// Options controls Add. Password is transient and never persisted by the caller.
type Options struct {
	Password        string
	DryRun          bool
	ResolveConflict bool
	ShortcutDir     string
}

// Result is a safe outcome. No password or raw API error is exposed.
type Result struct {
	Share           string   `json:"share"`
	UNC             string   `json:"unc"`
	Connected       bool     `json:"connected"`
	CredentialSaved bool     `json:"credential_saved"`
	CredentialName  string   `json:"credential_name,omitempty"`
	ShortcutPath    string   `json:"shortcut_path,omitempty"`
	Conflict        bool     `json:"conflict"`
	Warnings        []string `json:"warnings"`
}

// Manager coordinates credentials, connections and shortcuts.
type Manager struct {
	Store     CredentialStore
	Connector Connector
	Shortcuts ShortcutCreator
}

// Add establishes one share. A conflict (Error 1219) is returned as a typed error
// unless Options.ResolveConflict explicitly allows disconnecting stale sessions.
func (m *Manager) Add(ctx context.Context, share Share, options Options) (Result, error) {
	if err := share.Validate(); err != nil {
		return Result{}, err
	}
	if m.Connector == nil {
		return Result{}, fmt.Errorf("share connector is required")
	}
	if share.Shortcut && !options.DryRun && strings.TrimSpace(options.ShortcutDir) == "" {
		return Result{}, fmt.Errorf("shortcut directory is required")
	}
	server := share.Server()
	result := Result{Share: share.ID, UNC: share.UNC, Warnings: []string{}}
	target := CredentialTarget(server)
	credential := Credential{Username: share.Username, Password: options.Password}
	if options.DryRun {
		result.Warnings = append(result.Warnings, "Пробный режим: подключение не выполнялось")
		if share.Shortcut {
			result.ShortcutPath = plannedShortcutPath(share, options.ShortcutDir)
		}
		return result, nil
	}
	if credential.Password != "" {
		if strings.TrimSpace(credential.Username) == "" {
			return result, fmt.Errorf("user name is required together with a password")
		}
		if m.Store == nil {
			return result, fmt.Errorf("credential store is required for credentials")
		}
		if err := m.Store.Save(ctx, target, credential.Username, credential.Password); err != nil {
			return result, fmt.Errorf("could not save credentials in Windows Credential Manager")
		}
		result.CredentialSaved, result.CredentialName = true, target
	} else if m.Store != nil {
		if stored, ok, err := m.Store.Read(ctx, target); err == nil && ok {
			credential = stored
		}
	}
	if credential.Username != "" && credential.Password == "" {
		return result, fmt.Errorf("password is required together with a user name")
	}
	if options.ResolveConflict {
		if err := m.disconnectServer(ctx, server); err != nil {
			return result, err
		}
	}
	err := m.connect(ctx, share, credential, options)
	if errors.Is(err, ErrCredentialConflict) {
		result.Conflict = true
		return result, &ConflictError{Server: server, Existing: m.connectionsFor(ctx, server)}
	}
	if err != nil {
		return result, err
	}
	if err := m.confirmConnected(ctx, share.UNC); err != nil {
		return result, err
	}
	result.Connected = true
	if share.Shortcut {
		if m.Shortcuts == nil {
			result.Warnings = append(result.Warnings, "Ярлык не создан: компонент ярлыков недоступен")
		} else {
			path := plannedShortcutPath(share, options.ShortcutDir)
			if err := m.Shortcuts.Create(ctx, ShortcutSpec{Path: path, Target: share.UNC, Description: shortcutLabel(share)}); err != nil {
				result.Warnings = append(result.Warnings, "Не удалось создать ярлык на рабочем столе")
			} else {
				result.ShortcutPath = path
			}
		}
	}
	return result, nil
}

func (m *Manager) connect(ctx context.Context, share Share, credential Credential, options Options) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := m.Connector.Connect(ctx, ConnectionRequest{RemoteName: share.UNC, Username: credential.Username, Password: credential.Password, Persistent: share.Persistent})
	if errors.Is(err, ErrCredentialConflict) && options.ResolveConflict {
		if disconnectErr := m.disconnectServer(ctx, share.Server()); disconnectErr != nil {
			return disconnectErr
		}
		return m.Connector.Connect(ctx, ConnectionRequest{RemoteName: share.UNC, Username: credential.Username, Password: credential.Password, Persistent: share.Persistent})
	}
	return err
}

func (m *Manager) disconnectServer(ctx context.Context, server string) error {
	connections, err := m.Connector.List(ctx)
	if err != nil {
		return fmt.Errorf("could not enumerate existing connections")
	}
	for _, connection := range connections {
		if !sameServer(connection.RemoteName, server) {
			continue
		}
		if err := m.Connector.Disconnect(ctx, connection.RemoteName, true); err != nil {
			return fmt.Errorf("could not disconnect a conflicting connection")
		}
	}
	return nil
}

func (m *Manager) connectionsFor(ctx context.Context, server string) []string {
	connections, err := m.Connector.List(ctx)
	if err != nil {
		return nil
	}
	result := []string{}
	for _, connection := range connections {
		if sameServer(connection.RemoteName, server) {
			result = append(result, connection.RemoteName)
		}
	}
	return result
}

func (m *Manager) confirmConnected(ctx context.Context, unc string) error {
	connections, err := m.Connector.List(ctx)
	if err != nil {
		return fmt.Errorf("could not confirm the connection")
	}
	for _, connection := range connections {
		if strings.EqualFold(strings.TrimRight(connection.RemoteName, `\`), strings.TrimRight(unc, `\`)) {
			return nil
		}
	}
	return fmt.Errorf("connection was not confirmed by Windows")
}

func sameServer(remoteName, server string) bool {
	trimmed := strings.TrimPrefix(strings.TrimRight(remoteName, `\`), `\\`)
	if index := strings.Index(trimmed, `\`); index >= 0 {
		trimmed = trimmed[:index]
	}
	return strings.EqualFold(trimmed, server)
}

func shortcutLabel(share Share) string {
	if strings.TrimSpace(share.Name) != "" {
		return share.Name
	}
	return share.UNC
}

func plannedShortcutPath(share Share, directory string) string {
	base := strings.TrimSpace(share.Name)
	if base == "" {
		base = share.ID
	}
	safe := make([]rune, 0, len(base))
	for _, r := range base {
		switch {
		case r < 32, strings.ContainsRune(`<>:"/\|?*`, r):
			safe = append(safe, '_')
		default:
			safe = append(safe, r)
		}
	}
	name := strings.TrimSpace(string(safe))
	if name == "" {
		name = share.ID
	}
	if directory == "" {
		return ""
	}
	return filepath.Join(directory, name+".lnk")
}
