//go:build !windows

package settings

import "context"

// ErrUnsupported is returned by Windows adapters on other platforms.
var ErrUnsupportedPlatform = ErrUnsupported

type WindowsRegistry struct{}

func (WindowsRegistry) StringValue(context.Context, Root, string, string) (string, bool, error) {
	return "", false, ErrUnsupported
}
func (WindowsRegistry) DWordValue(context.Context, Root, string, string) (uint32, bool, error) {
	return 0, false, ErrUnsupported
}
func (WindowsRegistry) SetStringValue(context.Context, Root, string, string, string) error {
	return ErrUnsupported
}
func (WindowsRegistry) SetDWordValue(context.Context, Root, string, string, uint32) error {
	return ErrUnsupported
}
func (WindowsRegistry) DeleteValue(context.Context, Root, string, string) error {
	return ErrUnsupported
}
func (WindowsRegistry) KeyExists(context.Context, Root, string) (bool, error) {
	return false, ErrUnsupported
}
func (WindowsRegistry) ValueExists(context.Context, Root, string, string) (bool, error) {
	return false, ErrUnsupported
}

type WindowsRunner struct{}

func (WindowsRunner) Run(context.Context, string, ...string) (string, error) {
	return "", ErrUnsupported
}

type WindowsServices struct{}

func (WindowsServices) State(context.Context, string) (string, error) { return "", ErrUnsupported }

type WindowsComputerName struct{}

func (WindowsComputerName) SetComputerName(context.Context, string) error { return ErrUnsupported }

type WindowsActivation struct{}

func (WindowsActivation) Status(context.Context) (Activation, error) {
	return Activation{}, ErrUnsupported
}

type WindowsElevation struct{}

func (WindowsElevation) Elevated(context.Context) (bool, error) { return false, ErrUnsupported }
