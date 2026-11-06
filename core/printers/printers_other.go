//go:build !windows

package printers

import (
	"context"
	"errors"
)

// ErrUnsupported is returned by Windows adapters on other platforms.
var ErrUnsupported = errors.New("printer operations are supported on Windows only")

type WindowsPrinters struct{}

func (WindowsPrinters) Printers(context.Context) ([]Printer, error) { return nil, ErrUnsupported }

type WindowsDefaults struct{}

func (WindowsDefaults) Default(context.Context) (string, error)  { return "", ErrUnsupported }
func (WindowsDefaults) SetDefault(context.Context, string) error { return ErrUnsupported }

type WindowsInstaller struct{}

func (WindowsInstaller) AddPrinter(context.Context, AddRequest) error { return ErrUnsupported }

type WindowsTestPrinter struct{}

func (WindowsTestPrinter) PrintTestPage(context.Context, string) error { return ErrUnsupported }

type WindowsElevation struct{}

func (WindowsElevation) Elevated(context.Context) (bool, error) { return false, ErrUnsupported }
