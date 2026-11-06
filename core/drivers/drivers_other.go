//go:build !windows

package drivers

import (
	"context"
)

type WindowsDevices struct{}

func (WindowsDevices) Devices(context.Context) ([]Device, error) { return nil, ErrUnsupported }

type WindowsINFVerifier struct{}

func (WindowsINFVerifier) VerifyINF(context.Context, string) (Signature, error) {
	return Signature{}, ErrUnsupported
}

type WindowsInstaller struct{}

func (WindowsInstaller) Install(context.Context, DriverPackage) (InstallOutcome, error) {
	return InstallOutcome{}, ErrUnsupported
}

func (WindowsInstaller) ScanDevices(context.Context) error { return ErrUnsupported }

type WindowsRunner struct{}

func (WindowsRunner) Run(context.Context, Command) (Execution, error) {
	return Execution{}, ErrUnsupported
}

type WindowsElevation struct{}

func (WindowsElevation) Elevated(context.Context) (bool, error) { return false, ErrUnsupported }
