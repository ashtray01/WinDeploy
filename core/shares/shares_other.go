//go:build !windows

package shares

import "context"

// Windows adapters fail closed on other platforms; the domain model and manager
// remain fully testable with fakes.
type WindowsCredentialStore struct{}

func (WindowsCredentialStore) Save(context.Context, string, string, string) error {
	return ErrUnsupported
}
func (WindowsCredentialStore) Read(context.Context, string) (Credential, bool, error) {
	return Credential{}, false, ErrUnsupported
}
func (WindowsCredentialStore) Delete(context.Context, string) error { return ErrUnsupported }

type WindowsConnector struct{}

func (WindowsConnector) Connect(context.Context, ConnectionRequest) error {
	return ErrUnsupported
}
func (WindowsConnector) List(context.Context) ([]Connection, error) { return nil, ErrUnsupported }
func (WindowsConnector) Disconnect(context.Context, string, bool) error {
	return ErrUnsupported
}

type WindowsShortcut struct{}

func (WindowsShortcut) Create(context.Context, ShortcutSpec) error { return ErrUnsupported }

// DesktopDirectory is unavailable outside Windows.
func DesktopDirectory() (string, error) { return "", ErrUnsupported }
