package shares

import (
	"context"
	"errors"
	"testing"
)

type unconfirmedShortcut struct{}

func (unconfirmedShortcut) Create(context.Context, ShortcutSpec) error          { return nil }
func (unconfirmedShortcut) Matches(context.Context, ShortcutSpec) (bool, error) { return false, nil }

func TestShortcutWriteWithoutReadbackDoesNotReportSuccess(t *testing.T) {
	share := validShare()
	share.Username = ""
	manager := &Manager{Connector: &fakeConnector{}, Shortcuts: unconfirmedShortcut{}}
	result, err := manager.Add(context.Background(), share, Options{ShortcutDir: t.TempDir()})
	if !errors.Is(err, ErrShortcutRequired) || !result.Connected || result.ShortcutPath != "" {
		t.Fatalf("unconfirmed shortcut accepted: %+v err=%v", result, err)
	}
}

func TestExistingUNCShortcutRepairDoesNotReconnectOrSaveCredentials(t *testing.T) {
	share := validShare()
	share.Username = ""
	connector := &fakeConnector{connections: []Connection{{RemoteName: share.UNC}}}
	store := &fakeStore{}
	links := &fakeShortcut{}
	manager := &Manager{Connector: connector, Store: store, Shortcuts: links}
	result, err := manager.Add(context.Background(), share, Options{ShortcutDir: t.TempDir()})
	if err != nil || !result.Connected || result.ShortcutPath == "" || len(connector.requests) != 0 || len(store.saved) != 0 {
		t.Fatalf("repair mutated connection/credential: %+v %v", result, err)
	}
}
