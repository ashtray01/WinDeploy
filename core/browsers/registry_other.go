//go:build !windows

package browsers

import (
	"context"
	"errors"
)

// ErrUnsupported is returned by Windows adapters on other platforms.
var ErrUnsupported = errors.New("browser policies are supported on Windows only")

type RegistryStore struct{}

func NewRegistryStore() *RegistryStore { return &RegistryStore{} }
func (RegistryStore) WriteString(context.Context, Scope, Browser, string, string) error {
	return ErrUnsupported
}
func (RegistryStore) WriteDWord(context.Context, Scope, Browser, string, uint32) error {
	return ErrUnsupported
}
func (RegistryStore) DeleteValue(context.Context, Scope, Browser, string) error {
	return ErrUnsupported
}

type WindowsLocator struct{}

func (WindowsLocator) Executable(context.Context, Browser) (string, error) {
	return "", ErrUnsupported
}

type WindowsShortcut struct{}

func (WindowsShortcut) Create(context.Context, string, string, string, string) error {
	return ErrUnsupported
}

func DesktopDirectory() (string, error) { return "", ErrUnsupported }

type WindowsElevation struct{}

func (WindowsElevation) Elevated(context.Context) (bool, error) { return false, ErrUnsupported }
