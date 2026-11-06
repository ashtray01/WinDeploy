package service

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"windeploy/core/settings"
)

// WindowsSettingsView is the read-only settings snapshot.
type WindowsSettingsView struct {
	Hostname   string              `json:"hostname"`
	Timezone   string              `json:"timezone"`
	PowerPlan  string              `json:"power_plan"`
	Explorer   map[string]string   `json:"explorer"`
	Activation settings.Activation `json:"activation"`
	Elevated   bool                `json:"elevated"`
	Windows    string              `json:"windows"`
}

var guidReader = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

func (s *Service) settingsManager() *settings.Manager {
	system := s.currentSystem(context.Background())
	return &settings.Manager{
		Registry: settings.WindowsRegistry{}, Runner: settings.WindowsRunner{},
		Services: settings.WindowsServices{}, Elevation: settings.WindowsElevation{},
		Activation: settings.WindowsActivation{}, Computer: settings.WindowsComputerName{},
		System: system,
	}
}

// GetWindowsSettings reads selected settings without changing anything.
func (s *Service) GetWindowsSettings() (WindowsSettingsView, error) {
	manager := s.settingsManager()
	view := WindowsSettingsView{Explorer: map[string]string{}, Windows: "Windows"}
	view.Hostname = manager.System.Hostname
	switch manager.System.Windows.Token() {
	case "windows-11":
		view.Windows = "Windows 11"
	case "windows-10":
		view.Windows = "Windows 10"
	case "windows-7-sp1":
		view.Windows = "Windows 7 SP1"
	}
	if output, err := manager.Runner.Run(context.Background(), "tzutil.exe", "/g"); err == nil {
		view.Timezone = strings.TrimSpace(output)
	}
	if output, err := manager.Runner.Run(context.Background(), "powercfg.exe", "/getactivescheme"); err == nil {
		view.PowerPlan = strings.ToLower(guidReader.FindString(output))
	}
	if values, err := manager.ExplorerValues(context.Background()); err == nil {
		view.Explorer = values
	}
	if activation, err := manager.ActivationStatus(context.Background()); err == nil {
		view.Activation = activation
	} else {
		view.Activation = settings.Activation{Status: "unknown", Detail: "Статус лицензирования недоступен."}
	}
	if elevated, err := (settings.WindowsElevation{}).Elevated(context.Background()); err == nil {
		view.Elevated = elevated
	}
	return view, nil
}

// ApplyComputerName previews or renames the computer.
func (s *Service) ApplyComputerName(name string, dryRun bool) (settings.Result, error) {
	manager := s.settingsManager()
	result, err := manager.SetComputerName(context.Background(), name, dryRun)
	if err != nil {
		switch {
		case errors.Is(err, settings.ErrElevationRequired):
			return result, errors.New("Для переименования компьютера нужны права администратора.")
		default:
			return result, errors.New("Не удалось изменить имя компьютера. Проверьте имя и права.")
		}
	}
	return result, nil
}

// ApplyTimezone previews or changes the time zone.
func (s *Service) ApplyTimezone(timezoneID string, dryRun bool) (settings.Result, error) {
	result, err := s.settingsManager().SetTimezone(context.Background(), timezoneID, dryRun)
	if err != nil {
		return result, errors.New("Не удалось изменить часовой пояс. Проверьте идентификатор зоны Windows.")
	}
	return result, nil
}

// ApplyExplorer previews or writes Explorer options.
func (s *Service) ApplyExplorer(options settings.ExplorerOptions, dryRun bool) (settings.Result, error) {
	result, err := s.settingsManager().ApplyExplorer(context.Background(), options, dryRun)
	if err != nil {
		return result, errors.New("Не удалось изменить параметры Проводника.")
	}
	return result, nil
}

// ApplyPowerPlan previews or activates a Windows power scheme.
func (s *Service) ApplyPowerPlan(guid string, dryRun bool) (settings.Result, error) {
	result, err := s.settingsManager().SetPowerPlan(context.Background(), guid, dryRun)
	if err != nil {
		return result, errors.New("Не удалось изменить схему электропитания. Проверьте GUID.")
	}
	return result, nil
}

// GetWindowsUpdateDiagnostics runs read-only update diagnostics.
func (s *Service) GetWindowsUpdateDiagnostics() (settings.UpdateReport, error) {
	report, err := s.settingsManager().UpdateDiagnostics(context.Background())
	if err != nil {
		return report, errors.New("Не удалось выполнить диагностику Windows Update.")
	}
	return report, nil
}

// PowerPlanOption is one standard Windows power scheme.
type PowerPlanOption struct {
	GUID string `json:"guid"`
	Name string `json:"name"`
}

// GetPowerPlanOptions returns the standard Windows power schemes.
func (s *Service) GetPowerPlanOptions() []PowerPlanOption {
	return []PowerPlanOption{
		{GUID: "381b4222-f694-41f0-9685-ff5bb260df2e", Name: "Сбалансированная"},
		{GUID: "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c", Name: "Высокая производительность"},
		{GUID: "a1841308-3541-4fab-bc81-f71556f20b4a", Name: "Экономия энергии"},
	}
}
