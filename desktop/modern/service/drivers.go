package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"windeploy/core/drivers"
)

// DriverInstallRequest is one explicit driver installation request.
type DriverInstallRequest struct {
	INFPath    string `json:"inf_path"`
	HardwareID string `json:"hardware_id"`
	DryRun     bool   `json:"dry_run"`
}

// GetDevices enumerates present PnP devices read-only.
func (s *Service) GetDevices() ([]drivers.Device, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	devices, err := (drivers.WindowsDevices{}).Devices(ctx)
	if err != nil {
		return nil, errors.New("Не удалось перечислить устройства Windows.")
	}
	return devices, nil
}

// ScanDriverCatalog reads INF metadata from a local or network driver directory.
func (s *Service) ScanDriverCatalog(path string) ([]drivers.INFInfo, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("Укажите каталог драйверов.")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	packages, err := drivers.ScanDirectory(ctx, path, 2000)
	if err != nil {
		return nil, errors.New("Каталог драйверов недоступен или содержит ошибку.")
	}
	return packages, nil
}

// ScanDevicesForUpdates triggers a PnP driver rescan (Windows Update policy may
// supply matching drivers). The call requires administrator rights.
func (s *Service) ScanDevicesForUpdates() error {
	installer := drivers.WindowsInstaller{Runner: drivers.WindowsRunner{}, System: s.currentSystem(context.Background())}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	if err := installer.ScanDevices(ctx); err != nil {
		if errors.Is(err, drivers.ErrUnsupported) {
			return errors.New("Поиск драйверов недоступен на этой версии Windows.")
		}
		return errors.New("Не удалось запустить поиск драйверов. Проверьте права администратора.")
	}
	return nil
}

// InstallDriver verifies the INF signature and installs the driver with elevation.
func (s *Service) InstallDriver(request DriverInstallRequest) (drivers.Result, error) {
	manager := &drivers.Manager{
		Devices:   drivers.WindowsDevices{},
		Installer: drivers.WindowsInstaller{Runner: drivers.WindowsRunner{}, System: s.currentSystem(context.Background())},
		Verifier:  drivers.WindowsINFVerifier{},
		Elevation: drivers.WindowsElevation{},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
	defer cancel()
	result, err := manager.Install(ctx, drivers.InstallRequest{INFPath: request.INFPath, HardwareID: request.HardwareID, DryRun: request.DryRun})
	if err != nil {
		switch {
		case errors.Is(err, drivers.ErrUnsignedINF):
			return result, errors.New("Подпись INF-файла или каталога не принята Windows. Установка остановлена.")
		case errors.Is(err, drivers.ErrElevationRequired):
			return result, errors.New("Для установки драйвера нужны права администратора.")
		case strings.Contains(err.Error(), "not confirmed"):
			return result, errors.New("Драйвер установлен, но подходящее устройство не подтверждено. Проверьте диспетчер устройств.")
		default:
			return result, errors.New("Не удалось установить драйвер. Проверьте INF, подпись и права администратора.")
		}
	}
	return result, nil
}
