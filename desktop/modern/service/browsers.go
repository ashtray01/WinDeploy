package service

import (
	"context"
	"errors"
	"time"

	"windeploy/core/browsers"
)

// BrowserEnvironmentView lists detected browser executables and the desktop dir.
type BrowserEnvironmentView struct {
	Chrome   string `json:"chrome,omitempty"`
	Edge     string `json:"edge,omitempty"`
	Firefox  string `json:"firefox,omitempty"`
	Desktop  string `json:"desktop,omitempty"`
	Elevated bool   `json:"elevated"`
}

// GetBrowserEnvironment is read-only: it locates browsers without running them.
func (s *Service) GetBrowserEnvironment() BrowserEnvironmentView {
	view := BrowserEnvironmentView{}
	locator := browsers.WindowsLocator{}
	ctx := context.Background()
	if path, err := locator.Executable(ctx, browsers.Chrome); err == nil {
		view.Chrome = path
	}
	if path, err := locator.Executable(ctx, browsers.Edge); err == nil {
		view.Edge = path
	}
	if path, err := locator.Executable(ctx, browsers.Firefox); err == nil {
		view.Firefox = path
	}
	if desktop, err := browsers.DesktopDirectory(); err == nil {
		view.Desktop = desktop
	}
	if elevated, err := (browsers.WindowsElevation{}).Elevated(ctx); err == nil {
		view.Elevated = elevated
	}
	return view
}

// ApplyBrowserPolicy writes supported enterprise policies and optional shortcuts.
// It never edits existing browser profile databases.
func (s *Service) ApplyBrowserPolicy(policy browsers.Policy, dryRun bool) (browsers.Result, error) {
	if !dryRun && (policy.Scope != browsers.ScopeMachine || len(policy.WebApps) > 0) {
		if err := s.requireUserContext(context.Background()); err != nil {
			return browsers.Result{}, err
		}
	}
	desktop, _ := browsers.DesktopDirectory()
	manager := &browsers.Manager{
		Store:      browsers.NewRegistryStore(),
		Shortcuts:  browsers.WindowsShortcut{},
		Locator:    browsers.WindowsLocator{},
		Elevation:  browsers.WindowsElevation{},
		DesktopDir: desktop,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := manager.Apply(ctx, policy, dryRun)
	if err != nil {
		switch {
		case errors.Is(err, browsers.ErrWebAppsRequired):
			return result, errors.New("Не удалось создать обязательные ярлыки веб-приложений. Часть политик могла уже примениться; проверьте результат.")
		case errors.Is(err, browsers.ErrElevationRequired):
			return result, errors.New("Для политик уровня компьютера нужны права администратора.")
		default:
			return result, errors.New("Не удалось применить политику браузера. Проверьте URL, область действия и права.")
		}
	}
	return result, nil
}
