package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"windeploy/core/printers"
)

// GetPrinters enumerates installed print queues read-only.
func (s *Service) GetPrinters() ([]printers.Printer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	list, err := (printers.WindowsPrinters{}).Printers(ctx)
	if err != nil {
		return nil, errors.New("Не удалось перечислить принтеры Windows.")
	}
	return list, nil
}

// AddPrinter installs a print queue from an INF and a TCP/IP address.
func (s *Service) AddPrinter(request printers.AddRequest) (printers.Result, error) {
	manager := &printers.Manager{
		Printers:  printers.WindowsPrinters{},
		Installer: printers.WindowsInstaller{},
		Defaults:  printers.WindowsDefaults{},
		Tester:    printers.WindowsTestPrinter{},
		Elevation: printers.WindowsElevation{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	result, err := manager.Add(ctx, request)
	if err != nil {
		switch {
		case errors.Is(err, printers.ErrModelRequired):
			return result, errors.New("Выберите точную модель Kyocera 2460 из каталога перед установкой драйвера.")
		case errors.Is(err, printers.ErrElevationRequired):
			return result, errors.New("Для добавления принтера нужны права администратора.")
		default:
			return result, errors.New("Не удалось добавить принтер. Проверьте INF-драйвер, модель и IP-адрес.")
		}
	}
	return result, nil
}

// SetDefaultPrinter selects the current user's default printer.
func (s *Service) SetDefaultPrinter(name string) error {
	manager := &printers.Manager{Printers: printers.WindowsPrinters{}, Defaults: printers.WindowsDefaults{}}
	if err := manager.SetDefault(context.Background(), strings.TrimSpace(name)); err != nil {
		return errors.New("Не удалось изменить принтер по умолчанию.")
	}
	return nil
}

// PrintTestPage sends a real test page to an existing printer.
func (s *Service) PrintTestPage(name string) error {
	manager := &printers.Manager{Printers: printers.WindowsPrinters{}, Tester: printers.WindowsTestPrinter{}}
	if err := manager.TestPage(context.Background(), strings.TrimSpace(name)); err != nil {
		return errors.New("Не удалось отправить тестовую страницу на принтер.")
	}
	return nil
}

// GetKyocera2460Models returns the explicit model list required for the 2460 family.
func (s *Service) GetKyocera2460Models() []string {
	return append([]string{}, printers.Kyocera2460Models...)
}
