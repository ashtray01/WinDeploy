package shares

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

type fakeStore struct {
	credentials map[string]Credential
	saved       []string
	deleted     []string
	saveErr     error
}

func (s *fakeStore) Save(_ context.Context, target, username, password string) error {
	if s.saveErr != nil {
		return s.saveErr
	}
	if s.credentials == nil {
		s.credentials = map[string]Credential{}
	}
	s.credentials[target] = Credential{Username: username, Password: password}
	s.saved = append(s.saved, target)
	return nil
}

func (s *fakeStore) Read(_ context.Context, target string) (Credential, bool, error) {
	credential, ok := s.credentials[target]
	return credential, ok, nil
}

func (s *fakeStore) Delete(_ context.Context, target string) error {
	s.deleted = append(s.deleted, target)
	delete(s.credentials, target)
	return nil
}

type fakeConnector struct {
	connections    []Connection
	requests       []ConnectionRequest
	disconnected   []string
	connectErr     error
	confirmMissing bool
}

func (c *fakeConnector) Connect(_ context.Context, request ConnectionRequest) error {
	c.requests = append(c.requests, request)
	if c.connectErr != nil {
		return c.connectErr
	}
	c.connections = append(c.connections, Connection{RemoteName: request.RemoteName})
	return nil
}

func (c *fakeConnector) List(context.Context) ([]Connection, error) {
	if c.confirmMissing {
		return []Connection{}, nil
	}
	return append([]Connection{}, c.connections...), nil
}

func (c *fakeConnector) Disconnect(_ context.Context, remote string, _ bool) error {
	c.disconnected = append(c.disconnected, remote)
	filtered := c.connections[:0]
	for _, connection := range c.connections {
		if !strings.EqualFold(connection.RemoteName, remote) {
			filtered = append(filtered, connection)
		}
	}
	c.connections = filtered
	return nil
}

type fakeShortcut struct {
	specs []ShortcutSpec
	err   error
}

func (s *fakeShortcut) Create(_ context.Context, spec ShortcutSpec) error {
	s.specs = append(s.specs, spec)
	return s.err
}

func validShare() Share {
	return Share{ID: "finance", UNC: `\\fileserver\finance`, Name: "Финансы", Username: `OFFICE\user`, Shortcut: true}
}

func TestShareValidation(t *testing.T) {
	if err := validShare().Validate(); err != nil {
		t.Fatalf("valid share rejected: %v", err)
	}
	cases := []Share{
		{ID: "bad", UNC: `C:\temp`},
		{ID: "bad", UNC: `\\server`},
		{ID: "bad", UNC: `\\server\share\extra`},
		{ID: "bad", UNC: `\\user:pass@server\share`},
		{ID: "Bad ID", UNC: `\\server\share`},
	}
	for _, share := range cases {
		if err := share.Validate(); err == nil {
			t.Fatalf("invalid share accepted: %+v", share)
		}
	}
	if server := validShare().Server(); server != "fileserver" {
		t.Fatalf("server = %q", server)
	}
	if target := CredentialTarget("FileServer"); target != "WinDeploy:share:fileserver" {
		t.Fatalf("credential target = %q", target)
	}
}

func TestAddSavesCredentialConnectsAndCreatesShortcut(t *testing.T) {
	store, connector, shortcuts := &fakeStore{}, &fakeConnector{}, &fakeShortcut{}
	manager := &Manager{Store: store, Connector: connector, Shortcuts: shortcuts}
	desktop := t.TempDir()
	result, err := manager.Add(context.Background(), validShare(), Options{Password: "secret", ShortcutDir: desktop})
	if err != nil {
		t.Fatalf("add failed: %v", err)
	}
	if !result.Connected || !result.CredentialSaved || result.ShortcutPath == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(store.saved) != 1 || store.saved[0] != "WinDeploy:share:fileserver" {
		t.Fatalf("credential was not stored per server: %+v", store.saved)
	}
	if len(connector.requests) != 1 {
		t.Fatalf("expected one connection attempt, got %d", len(connector.requests))
	}
	request := connector.requests[0]
	if request.RemoteName != `\\fileserver\finance` || request.Username != `OFFICE\user` || request.Password != "secret" || request.Persistent {
		t.Fatalf("unexpected connection request: %+v", request)
	}
	if len(shortcuts.specs) != 1 {
		t.Fatalf("expected one shortcut, got %d", len(shortcuts.specs))
	}
	expectedPath := filepath.Join(desktop, "Финансы.lnk")
	if shortcuts.specs[0].Path != expectedPath || shortcuts.specs[0].Target != `\\fileserver\finance` {
		t.Fatalf("unexpected shortcut: %+v", shortcuts.specs[0])
	}
}

func TestAddUsesStoredCredentialWithoutPassword(t *testing.T) {
	store := &fakeStore{credentials: map[string]Credential{"WinDeploy:share:fileserver": {Username: `OFFICE\stored`, Password: "stored-password"}}}
	connector := &fakeConnector{}
	manager := &Manager{Store: store, Connector: connector}
	share := validShare()
	share.Shortcut = false
	result, err := manager.Add(context.Background(), share, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Connected || result.CredentialSaved {
		t.Fatalf("unexpected result: %+v", result)
	}
	if connector.requests[0].Username != `OFFICE\stored` || connector.requests[0].Password != "stored-password" {
		t.Fatalf("stored credential was not used: %+v", connector.requests[0])
	}
}

func TestAddReportsCredentialConflict(t *testing.T) {
	connector := &fakeConnector{connectErr: ErrCredentialConflict, connections: []Connection{{RemoteName: `\\fileserver\other`}}}
	manager := &Manager{Connector: connector}
	share := validShare()
	share.Username = ""
	share.Shortcut = false
	_, err := manager.Add(context.Background(), share, Options{})
	if !errors.Is(err, ErrCredentialConflict) {
		t.Fatalf("expected conflict error, got %v", err)
	}
	var conflict *ConflictError
	if !errors.As(err, &conflict) || conflict.Server != "fileserver" {
		t.Fatalf("unexpected conflict details: %+v", conflict)
	}
	if len(connector.disconnected) != 0 {
		t.Fatal("conflict resolution must not run without explicit consent")
	}
}

func TestAddResolvesConflictExplicitly(t *testing.T) {
	connector := &fakeConnector{connections: []Connection{{RemoteName: `\\fileserver\finance`}, {RemoteName: `\\other\share`}}}
	manager := &Manager{Connector: connector, Shortcuts: &fakeShortcut{}}
	desktop := t.TempDir()
	share := validShare()
	share.Username = ""
	result, err := manager.Add(context.Background(), share, Options{ResolveConflict: true, ShortcutDir: desktop})
	if err != nil {
		t.Fatal(err)
	}
	if !result.Connected {
		t.Fatalf("expected connection after conflict resolution: %+v", result)
	}
	if len(connector.disconnected) != 1 || connector.disconnected[0] != `\\fileserver\finance` {
		t.Fatalf("unexpected disconnected connections: %+v", connector.disconnected)
	}
}

func TestAddDryRunChangesNothing(t *testing.T) {
	store, connector, shortcuts := &fakeStore{}, &fakeConnector{}, &fakeShortcut{}
	manager := &Manager{Store: store, Connector: connector, Shortcuts: shortcuts}
	desktop := t.TempDir()
	result, err := manager.Add(context.Background(), validShare(), Options{DryRun: true, ShortcutDir: desktop})
	if err != nil {
		t.Fatal(err)
	}
	if result.Connected || len(store.saved) != 0 || len(connector.requests) != 0 || len(shortcuts.specs) != 0 {
		t.Fatalf("dry run changed state: %+v", result)
	}
	if result.ShortcutPath != filepath.Join(desktop, "Финансы.lnk") {
		t.Fatalf("unexpected planned shortcut: %q", result.ShortcutPath)
	}
}

func TestAddRequiresShortcutDirectoryAndConfirmation(t *testing.T) {
	manager := &Manager{Connector: &fakeConnector{}, Shortcuts: &fakeShortcut{}}
	if _, err := manager.Add(context.Background(), validShare(), Options{}); err == nil {
		t.Fatal("shortcut without a directory must fail")
	}
	connector := &fakeConnector{confirmMissing: true}
	manager = &Manager{Connector: connector}
	share := validShare()
	share.Shortcut = false
	share.Username = ""
	if _, err := manager.Add(context.Background(), share, Options{}); err == nil {
		t.Fatal("unconfirmed connection must fail")
	}
}

func TestAddCredentialStoreFailureStopsConnection(t *testing.T) {
	store := &fakeStore{saveErr: errors.New("dpapi unavailable")}
	connector := &fakeConnector{}
	manager := &Manager{Store: store, Connector: connector}
	if _, err := manager.Add(context.Background(), validShare(), Options{Password: "secret"}); err == nil {
		t.Fatal("credential store failure must stop the operation")
	}
	if len(connector.requests) != 0 {
		t.Fatal("connection must not be attempted without stored credentials")
	}
}

func TestAddShortcutFailureIsWarningOnly(t *testing.T) {
	connector := &fakeConnector{}
	manager := &Manager{Connector: connector, Shortcuts: &fakeShortcut{err: errors.New("denied")}}
	share := validShare()
	share.Username = ""
	result, err := manager.Add(context.Background(), share, Options{ShortcutDir: t.TempDir()})
	if err != nil {
		t.Fatalf("shortcut failure should not fail the connection: %v", err)
	}
	if !result.Connected || len(result.Warnings) == 0 {
		t.Fatalf("unexpected result: %+v", result)
	}
}
