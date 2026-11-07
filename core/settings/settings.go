// Package settings reads and changes selected Windows workstation settings with
// previews, explicit confirmation and restore data. Licensing is read-only: the
// package never bypasses activation.
package settings

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"windeploy/core/inventory"
)

var hostnamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]{0,62}$`)
var timezoneIDPattern = regexp.MustCompile(`^[A-Za-z0-9_+\- ]{1,128}$`)
var guidPattern = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
var guidInText = regexp.MustCompile(`[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}`)

// Change is one previewable setting change.
type Change struct {
	Name     string `json:"name"`
	OldValue string `json:"old_value"`
	NewValue string `json:"new_value"`
	Action   string `json:"action"`
}

// Result is a safe settings outcome.
type Result struct {
	Changes        []Change `json:"changes"`
	Applied        bool     `json:"applied"`
	RebootRequired bool     `json:"reboot_required"`
	Warnings       []string `json:"warnings"`
	Message        string   `json:"message"`
}

// Runner executes Windows helper tools and captures output.
type Runner interface {
	Run(context.Context, string, ...string) (string, error)
}

// Registry reads and writes specific registry values.
type Registry interface {
	StringValue(ctx context.Context, root Root, path, name string) (string, bool, error)
	DWordValue(ctx context.Context, root Root, path, name string) (uint32, bool, error)
	SetStringValue(ctx context.Context, root Root, path, name, value string) error
	SetDWordValue(ctx context.Context, root Root, path, name string, value uint32) error
	DeleteValue(ctx context.Context, root Root, path, name string) error
	KeyExists(ctx context.Context, root Root, path string) (bool, error)
	ValueExists(ctx context.Context, root Root, path, name string) (bool, error)
}

// ComputerNameSetter performs the native rename operation.
type ComputerNameSetter interface {
	SetComputerName(ctx context.Context, name string) error
}

// Root identifies the registry hive.
type Root string

const (
	RootUser    Root = "HKCU"
	RootMachine Root = "HKLM"
)

// Elevation reports administrator rights.
type Elevation interface {
	Elevated(ctx context.Context) (bool, error)
}

// ServiceQuery reads Windows service state.
type ServiceQuery interface {
	State(ctx context.Context, name string) (string, error)
}

// ActivationProbe reads Windows activation/genuine state read-only.
type ActivationProbe interface {
	Status(ctx context.Context) (Activation, error)
}

// Activation is a read-only licensing snapshot.
type Activation struct {
	Genuine bool   `json:"genuine"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

// ServiceState is the update service snapshot.
type ServiceState struct {
	Name  string `json:"name"`
	State string `json:"state"`
}

// UpdateReport is a read-only Windows Update diagnostic.
type UpdateReport struct {
	Checks   []Check        `json:"checks"`
	Services []ServiceState `json:"services"`
	Message  string         `json:"message"`
}

// Check is one diagnostic result.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// ExplorerOptions selects Explorer settings; nil fields are left unchanged.
type ExplorerOptions struct {
	ShowHidden      *bool `json:"show_hidden,omitempty"`
	ShowExtensions  *bool `json:"show_extensions,omitempty"`
	ShowSuperHidden *bool `json:"show_super_hidden,omitempty"`
	OpenThisPC      *bool `json:"open_this_pc,omitempty"`
}

// Manager applies supported Windows settings.
type Manager struct {
	Registry   Registry
	Runner     Runner
	Services   ServiceQuery
	Elevation  Elevation
	Activation ActivationProbe
	Computer   ComputerNameSetter
	System     inventory.SystemInfo
}

// ErrElevationRequired is returned for machine changes without elevation.
var ErrElevationRequired = errors.New("administrator privileges are required")

// ErrUnsupported is returned when a setting is not supported on this Windows.
var ErrUnsupported = errors.New("setting is unsupported on this Windows version")

// PreviewComputerName validates and previews a computer rename.
func PreviewComputerName(current, name string) (Change, error) {
	name = strings.TrimSpace(name)
	if !hostnamePattern.MatchString(name) {
		return Change{}, fmt.Errorf("invalid computer name: use 1-63 letters, digits or hyphens")
	}
	if strings.EqualFold(name, current) {
		return Change{}, fmt.Errorf("new computer name matches the current name")
	}
	return Change{Name: "computer-name", OldValue: current, NewValue: name, Action: "planned"}, nil
}

// SetComputerName renames the computer. A reboot is always required.
func (m *Manager) SetComputerName(ctx context.Context, name string, dryRun bool) (Result, error) {
	current := m.System.Hostname
	change, err := PreviewComputerName(current, name)
	if err != nil {
		return Result{}, err
	}
	result := Result{Changes: []Change{change}, Warnings: []string{}}
	if dryRun {
		result.Message = "Проверка выполнена: имя компьютера не изменялось."
		return result, nil
	}
	if err := m.requireElevation(ctx); err != nil {
		return result, err
	}
	if m.Computer == nil {
		return result, ErrUnsupported
	}
	if err := m.Computer.SetComputerName(ctx, change.NewValue); err != nil {
		return result, fmt.Errorf("computer name could not be changed")
	}
	result.Applied, result.RebootRequired = true, true
	result.Message = "Имя компьютера изменено. Требуется перезагрузка."
	return result, nil
}

// SetTimezone validates a time zone ID and applies it through tzutil.
func (m *Manager) SetTimezone(ctx context.Context, timezoneID string, dryRun bool) (Result, error) {
	timezoneID = strings.TrimSpace(timezoneID)
	if !timezoneIDPattern.MatchString(timezoneID) {
		return Result{}, fmt.Errorf("invalid time zone ID")
	}
	if m.Registry == nil {
		return Result{}, ErrUnsupported
	}
	exists, err := m.Registry.KeyExists(ctx, RootMachine, `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Time Zones\`+timezoneID)
	if err != nil || !exists {
		return Result{}, fmt.Errorf("unknown time zone ID")
	}
	current := ""
	if active, err := m.TimezoneID(ctx); err == nil {
		current = active
	}
	change := Change{Name: "timezone", OldValue: current, NewValue: timezoneID, Action: "planned"}
	result := Result{Changes: []Change{change}, Warnings: []string{}}
	if dryRun {
		result.Message = "Проверка выполнена: часовой пояс не изменялся."
		return result, nil
	}
	if m.Runner == nil {
		return result, ErrUnsupported
	}
	if _, err := m.Runner.Run(ctx, "tzutil.exe", "/s", timezoneID); err != nil {
		return result, fmt.Errorf("time zone could not be changed")
	}
	change.Action = "applied"
	result.Changes = []Change{change}
	result.Applied = true
	// The command exit code is not evidence: read the active zone back.
	if active, err := m.TimezoneID(ctx); err != nil || !strings.EqualFold(active, timezoneID) {
		result.Warnings = append(result.Warnings, "Часовой пояс изменён, но не подтверждён чтением.")
	}
	result.Message = "Часовой пояс изменён."
	return result, nil
}

// TimezoneID reads the active Windows time zone identifier through tzutil.
func (m *Manager) TimezoneID(ctx context.Context) (string, error) {
	if m.Runner == nil {
		return "", ErrUnsupported
	}
	output, err := m.Runner.Run(ctx, "tzutil.exe", "/g")
	if err != nil {
		return "", fmt.Errorf("time zone could not be read")
	}
	id := strings.TrimSpace(output)
	if !timezoneIDPattern.MatchString(id) {
		return "", fmt.Errorf("time zone response is invalid")
	}
	return id, nil
}

// VerifyTimezone confirms the active time zone matches the expected identifier.
func (m *Manager) VerifyTimezone(ctx context.Context, timezoneID string) error {
	active, err := m.TimezoneID(ctx)
	if err != nil {
		return err
	}
	if !strings.EqualFold(active, strings.TrimSpace(timezoneID)) {
		return fmt.Errorf("time zone was not confirmed")
	}
	return nil
}

// ExplorerValues returns the current Explorer option values.
func (m *Manager) ExplorerValues(ctx context.Context) (map[string]string, error) {
	if m.Registry == nil {
		return nil, ErrUnsupported
	}
	values := map[string]string{}
	for name := range explorerValueNames {
		if value, ok, err := m.Registry.DWordValue(ctx, RootUser, explorerPath, name); err == nil && ok {
			values[name] = fmt.Sprint(value)
		}
	}
	return values, nil
}

const explorerPath = `Software\Microsoft\Windows\CurrentVersion\Explorer\Advanced`

var explorerValueNames = map[string]string{
	"Hidden":          "show-hidden",
	"HideFileExt":     "hide-extensions",
	"ShowSuperHidden": "show-super-hidden",
	"LaunchTo":        "open-this-pc",
}

// explorerWrite is one planned Explorer registry value.
type explorerWrite struct {
	name  string
	value uint32
	label string
}

func explorerWrites(options ExplorerOptions) ([]explorerWrite, error) {
	writes := []explorerWrite{}
	if options.ShowHidden != nil {
		value := uint32(2)
		if *options.ShowHidden {
			value = 1
		}
		writes = append(writes, explorerWrite{"Hidden", value, "show-hidden"})
	}
	if options.ShowExtensions != nil {
		value := uint32(1)
		if *options.ShowExtensions {
			value = 0
		}
		writes = append(writes, explorerWrite{"HideFileExt", value, "show-extensions"})
	}
	if options.ShowSuperHidden != nil {
		value := uint32(0)
		if *options.ShowSuperHidden {
			value = 1
		}
		writes = append(writes, explorerWrite{"ShowSuperHidden", value, "show-super-hidden"})
	}
	if options.OpenThisPC != nil {
		value := uint32(2)
		if *options.OpenThisPC {
			value = 1
		}
		writes = append(writes, explorerWrite{"LaunchTo", value, "open-this-pc"})
	}
	if len(writes) == 0 {
		return nil, fmt.Errorf("no Explorer options selected")
	}
	return writes, nil
}

// ApplyExplorer writes selected Explorer values, returns previous values for
// restore and confirms every written value by reading it back. Explorer is not
// restarted automatically.
func (m *Manager) ApplyExplorer(ctx context.Context, options ExplorerOptions, dryRun bool) (Result, error) {
	if m.Registry == nil {
		return Result{}, ErrUnsupported
	}
	writes, err := explorerWrites(options)
	if err != nil {
		return Result{}, err
	}
	result := Result{Changes: []Change{}, Warnings: []string{}}
	for _, item := range writes {
		old := ""
		if value, ok, err := m.Registry.DWordValue(ctx, RootUser, explorerPath, item.name); err == nil && ok {
			old = fmt.Sprint(value)
		}
		result.Changes = append(result.Changes, Change{Name: item.label, OldValue: old, NewValue: fmt.Sprint(item.value), Action: "planned"})
	}
	if dryRun {
		result.Message = "Проверка выполнена: параметры Проводника не изменялись."
		return result, nil
	}
	for index, item := range writes {
		if err := m.Registry.SetDWordValue(ctx, RootUser, explorerPath, item.name, item.value); err != nil {
			return result, fmt.Errorf("Explorer option could not be changed")
		}
		result.Changes[index].Action = "applied"
	}
	result.Applied = true
	if err := m.VerifyExplorer(ctx, options); err != nil {
		result.Warnings = append(result.Warnings, "Параметры Проводника записаны, но не подтверждены чтением.")
	}
	result.Warnings = append(result.Warnings, "Изменения появятся после обновления окна Проводника или повторного входа.")
	result.Message = "Параметры Проводника изменены."
	return result, nil
}

// VerifyExplorer confirms every requested Explorer value by reading it back.
func (m *Manager) VerifyExplorer(ctx context.Context, options ExplorerOptions) error {
	if m.Registry == nil {
		return ErrUnsupported
	}
	writes, err := explorerWrites(options)
	if err != nil {
		return err
	}
	for _, item := range writes {
		value, ok, err := m.Registry.DWordValue(ctx, RootUser, explorerPath, item.name)
		if err != nil || !ok || value != item.value {
			return fmt.Errorf("Explorer option was not confirmed")
		}
	}
	return nil
}

// SetPowerPlan activates a known Windows power scheme GUID.
func (m *Manager) SetPowerPlan(ctx context.Context, guid string, dryRun bool) (Result, error) {
	guid = strings.TrimSpace(guid)
	if !guidPattern.MatchString(guid) {
		return Result{}, fmt.Errorf("invalid power scheme GUID")
	}
	if m.Registry == nil {
		return Result{}, ErrUnsupported
	}
	exists, err := m.Registry.KeyExists(ctx, RootMachine, `SYSTEM\CurrentControlSet\Control\Power\User\PowerSchemes\`+guid)
	if err != nil || !exists {
		return Result{}, fmt.Errorf("unknown power scheme GUID")
	}
	current := ""
	if active, err := m.ActivePowerPlan(ctx); err == nil {
		current = active
	}
	change := Change{Name: "power-plan", OldValue: current, NewValue: strings.ToLower(guid), Action: "planned"}
	result := Result{Changes: []Change{change}, Warnings: []string{}}
	if dryRun {
		result.Message = "Проверка выполнена: схема электропитания не изменялась."
		return result, nil
	}
	if m.Runner == nil {
		return result, ErrUnsupported
	}
	if _, err := m.Runner.Run(ctx, "powercfg.exe", "/setactive", guid); err != nil {
		return result, fmt.Errorf("power scheme could not be activated")
	}
	change.Action = "applied"
	result.Changes = []Change{change}
	result.Applied = true
	if active, err := m.ActivePowerPlan(ctx); err != nil || active != strings.ToLower(guid) {
		result.Warnings = append(result.Warnings, "Схема электропитания изменена, но не подтверждена чтением.")
	}
	result.Message = "Схема электропитания изменена."
	return result, nil
}

// ActivePowerPlan reads the active scheme GUID through powercfg.
func (m *Manager) ActivePowerPlan(ctx context.Context) (string, error) {
	if m.Runner == nil {
		return "", ErrUnsupported
	}
	output, err := m.Runner.Run(ctx, "powercfg.exe", "/getactivescheme")
	if err != nil {
		return "", fmt.Errorf("power scheme could not be read")
	}
	match := guidInText.FindString(output)
	if match == "" {
		return "", fmt.Errorf("power scheme response is invalid")
	}
	return strings.ToLower(match), nil
}

// VerifyPowerPlan confirms the active scheme matches the expected GUID.
func (m *Manager) VerifyPowerPlan(ctx context.Context, guid string) error {
	if !guidPattern.MatchString(strings.TrimSpace(guid)) {
		return fmt.Errorf("invalid power scheme GUID")
	}
	active, err := m.ActivePowerPlan(ctx)
	if err != nil {
		return err
	}
	if active != strings.ToLower(strings.TrimSpace(guid)) {
		return fmt.Errorf("power scheme was not confirmed")
	}
	return nil
}

// UpdateDiagnostics checks update service state, pending reboot markers and
// Windows Update policy without changing anything.
func (m *Manager) UpdateDiagnostics(ctx context.Context) (UpdateReport, error) {
	report := UpdateReport{Checks: []Check{}, Services: []ServiceState{}, Message: "Диагностика завершена."}
	if m.Services != nil {
		state, err := m.Services.State(ctx, "wuauserv")
		if err != nil {
			report.Checks = append(report.Checks, Check{Name: "Служба Windows Update", Status: "failed", Detail: "Состояние недоступно"})
		} else {
			report.Services = append(report.Services, ServiceState{Name: "wuauserv", State: state})
			status := "ok"
			if state != "running" {
				status = "warning"
			}
			report.Checks = append(report.Checks, Check{Name: "Служба Windows Update", Status: status, Detail: state})
		}
	}
	pending := []string{}
	if m.Registry != nil {
		markers := []struct {
			root Root
			path string
			name string
		}{
			{RootMachine, `SOFTWARE\Microsoft\Windows\CurrentVersion\WindowsUpdate\Auto Update\RebootRequired`, ""},
			{RootMachine, `SOFTWARE\Microsoft\Windows\CurrentVersion\Component Based Servicing\RebootPending`, ""},
			{RootMachine, `SYSTEM\CurrentControlSet\Control\Session Manager`, "PendingFileRenameOperations"},
		}
		for _, marker := range markers {
			exists := false
			var err error
			if marker.name == "" {
				exists, err = m.Registry.KeyExists(ctx, marker.root, marker.path)
			} else {
				exists, err = m.Registry.ValueExists(ctx, marker.root, marker.path, marker.name)
			}
			if err == nil && exists {
				pending = append(pending, marker.path)
			}
		}
		policies, policyErr := m.Registry.KeyExists(ctx, RootMachine, `SOFTWARE\Policies\Microsoft\Windows\WindowsUpdate`)
		if policyErr == nil && policies {
			report.Checks = append(report.Checks, Check{Name: "Политика Windows Update", Status: "warning", Detail: "Обнаружены корпоративные политики WSUS/обновлений"})
		}
	}
	if len(pending) > 0 {
		report.Checks = append(report.Checks, Check{Name: "Ожидается перезагрузка", Status: "warning", Detail: "Windows ожидает перезагрузку после обновлений"})
	} else {
		report.Checks = append(report.Checks, Check{Name: "Ожидается перезагрузка", Status: "ok", Detail: "Маркеры перезагрузки не найдены"})
	}
	return report, nil
}

// ActivationStatus reads Windows activation state without modifying licensing.
func (m *Manager) ActivationStatus(ctx context.Context) (Activation, error) {
	if m.Activation == nil {
		return Activation{}, ErrUnsupported
	}
	return m.Activation.Status(ctx)
}

func (m *Manager) requireElevation(ctx context.Context) error {
	if m.Elevation == nil {
		return ErrElevationRequired
	}
	elevated, err := m.Elevation.Elevated(ctx)
	if err != nil || !elevated {
		return ErrElevationRequired
	}
	return nil
}
