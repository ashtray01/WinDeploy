// Package printers enumerates Windows printers, installs print queues from INF
// packages and supports TCP/IP ports, default printer selection and test pages.
// Modifying operations require elevation and an explicit request.
package printers

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Printer is one installed print queue.
type Printer struct {
	Name       string `json:"name"`
	Driver     string `json:"driver,omitempty"`
	Port       string `json:"port,omitempty"`
	Server     string `json:"server,omitempty"`
	Shared     bool   `json:"shared"`
	ShareName  string `json:"share_name,omitempty"`
	Default    bool   `json:"default"`
	Status     string `json:"status"`
	StatusCode uint32 `json:"status_code,omitempty"`
}

// Provider enumerates printers read-only.
type Provider interface {
	Printers(context.Context) ([]Printer, error)
}

// Installer adds a print queue.
type Installer interface {
	AddPrinter(context.Context, AddRequest) error
}

// DefaultManager reads and changes the per-user default printer.
type DefaultManager interface {
	Default(context.Context) (string, error)
	SetDefault(context.Context, string) error
}

// TestPrinter prints a test page.
type TestPrinter interface {
	PrintTestPage(context.Context, string) error
}

// Elevation reports administrator rights.
type Elevation interface {
	Elevated(context.Context) (bool, error)
}

// ErrElevationRequired is returned before installing a queue without elevation.
var ErrElevationRequired = errors.New("administrator privileges are required")

// ErrModelRequired is returned for Kyocera 2460 requests without an explicit,
// catalogued model.
var ErrModelRequired = errors.New("exact printer model is required")

// AddRequest describes one print queue installation.
type AddRequest struct {
	Name       string `json:"name"`
	INFPath    string `json:"inf_path"`
	Model      string `json:"model"`
	IPAddress  string `json:"ip_address"`
	Share      bool   `json:"share"`
	ShareName  string `json:"share_name"`
	SetDefault bool   `json:"set_default"`
	DryRun     bool   `json:"dry_run"`
}

// Result is a safe printer operation outcome.
type Result struct {
	Name         string   `json:"name"`
	Installed    bool     `json:"installed"`
	Default      bool     `json:"default"`
	TestPrinted  bool     `json:"test_printed"`
	RebootNeeded bool     `json:"reboot_needed"`
	Printer      *Printer `json:"printer,omitempty"`
	Message      string   `json:"message"`
	Warnings     []string `json:"warnings"`
}

// Manager coordinates validation, installation, default selection and test pages.
type Manager struct {
	Printers  Provider
	Installer Installer
	Defaults  DefaultManager
	Tester    TestPrinter
	Elevation Elevation
}

// ValidateAddRequest checks all fields without touching the system.
func ValidateAddRequest(request AddRequest) error {
	if strings.TrimSpace(request.Name) == "" || len(request.Name) > 220 || strings.ContainsAny(request.Name, "\x00\r\n") {
		return fmt.Errorf("printer name is required")
	}
	if strings.TrimSpace(request.INFPath) == "" {
		return fmt.Errorf("driver INF path is required")
	}
	info, err := os.Stat(request.INFPath)
	if err != nil || !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(request.INFPath), ".inf") {
		return fmt.Errorf("driver INF file is unavailable")
	}
	ip := net.ParseIP(strings.TrimSpace(request.IPAddress))
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("IPv4 printer address is required")
	}
	if request.Share && strings.TrimSpace(request.ShareName) == "" {
		return fmt.Errorf("share name is required for a shared printer")
	}
	return ValidateModel(request.Model, IsKyocera2460(request))
}

// IsKyocera2460 reports whether the request/model family targets Kyocera 2460.
func IsKyocera2460(request AddRequest) bool {
	haystack := strings.ToLower(request.Name + " " + request.Model + " " + request.INFPath)
	return strings.Contains(haystack, "kyocera") && (strings.Contains(haystack, "2460") || strings.Contains(haystack, "taskalfa"))
}

// Kyocera2460Models is the configurable model list for the 2460 family. The model
// must be chosen explicitly; the installer never guesses.
var Kyocera2460Models = []string{"TASKalfa 2460ci", "TASKalfa 2460ci+"}

// ValidateModel enforces explicit model selection for the Kyocera 2460 family.
func ValidateModel(model string, kyocera2460 bool) error {
	model = strings.TrimSpace(model)
	if !kyocera2460 {
		if model == "" {
			return fmt.Errorf("printer driver model is required")
		}
		return nil
	}
	if model == "" {
		return fmt.Errorf("%w: выберите точную модель Kyocera 2460", ErrModelRequired)
	}
	for _, allowed := range Kyocera2460Models {
		if strings.EqualFold(model, allowed) {
			return nil
		}
	}
	return fmt.Errorf("%w: модель %q отсутствует в каталоге Kyocera 2460", ErrModelRequired, model)
}

// Add installs a print queue and confirms it through enumeration.
func (m *Manager) Add(ctx context.Context, request AddRequest) (Result, error) {
	result := Result{Name: strings.TrimSpace(request.Name), Warnings: []string{}}
	if err := ValidateAddRequest(request); err != nil {
		return result, err
	}
	if m.Installer == nil {
		return result, fmt.Errorf("printer installer is required")
	}
	if !request.DryRun {
		if m.Elevation == nil {
			return result, ErrElevationRequired
		}
		elevated, err := m.Elevation.Elevated(ctx)
		if err != nil || !elevated {
			return result, ErrElevationRequired
		}
	}
	if request.DryRun {
		result.Message = "Проверка выполнена: принтер не добавлялся."
		return result, nil
	}
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if err := m.Installer.AddPrinter(runCtx, request); err != nil {
		return result, fmt.Errorf("printer installation failed")
	}
	if m.Printers != nil {
		printers, err := m.Printers.Printers(ctx)
		if err == nil {
			for i := range printers {
				if strings.EqualFold(printers[i].Name, result.Name) {
					printer := printers[i]
					result.Printer = &printer
					result.Installed = true
					break
				}
			}
		}
	}
	if !result.Installed {
		return result, fmt.Errorf("installed printer was not confirmed")
	}
	// The installer creates the queue on an IP_<address> port. A different port means
	// the queue points somewhere else even though the driver reported success.
	if request.IPAddress != "" && !strings.EqualFold(result.Printer.Port, "IP_"+strings.TrimSpace(request.IPAddress)) {
		return result, fmt.Errorf("installed printer uses an unexpected port")
	}
	if model := strings.TrimSpace(request.Model); model != "" && !strings.Contains(strings.ToLower(result.Printer.Driver), strings.ToLower(model)) {
		result.Warnings = append(result.Warnings, "Драйвер принтера не совпадает с выбранной моделью; проверьте очередь.")
	}
	if request.SetDefault {
		if m.Defaults == nil {
			result.Warnings = append(result.Warnings, "Принтер по умолчанию не изменён: компонент недоступен")
		} else if err := m.Defaults.SetDefault(ctx, result.Name); err != nil {
			result.Warnings = append(result.Warnings, "Не удалось назначить принтер по умолчанию")
		} else if confirmed, err := m.Defaults.Default(ctx); err != nil || !strings.EqualFold(confirmed, result.Name) {
			result.Warnings = append(result.Warnings, "Принтер по умолчанию назначен, но не подтверждён")
		} else {
			result.Default = true
		}
	}
	result.Message = "Принтер добавлен."
	return result, nil
}

// SetDefault selects an existing printer as the current user's default.
func (m *Manager) SetDefault(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("printer name is required")
	}
	if m.Printers == nil || m.Defaults == nil {
		return fmt.Errorf("printer components are unavailable")
	}
	printers, err := m.Printers.Printers(ctx)
	if err != nil {
		return fmt.Errorf("printers are unavailable")
	}
	for _, printer := range printers {
		if strings.EqualFold(printer.Name, name) {
			if err := m.Defaults.SetDefault(ctx, printer.Name); err != nil {
				return fmt.Errorf("default printer could not be changed")
			}
			// The call is not evidence: read the default back.
			confirmed, err := m.Defaults.Default(ctx)
			if err != nil || !strings.EqualFold(confirmed, printer.Name) {
				return fmt.Errorf("default printer was not confirmed")
			}
			return nil
		}
	}
	return fmt.Errorf("printer was not found")
}

// TestPage prints a real test page for an existing printer.
func (m *Manager) TestPage(ctx context.Context, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("printer name is required")
	}
	if m.Printers == nil || m.Tester == nil {
		return fmt.Errorf("printer components are unavailable")
	}
	printers, err := m.Printers.Printers(ctx)
	if err != nil {
		return fmt.Errorf("printers are unavailable")
	}
	for _, printer := range printers {
		if strings.EqualFold(printer.Name, name) {
			if err := m.Tester.PrintTestPage(ctx, printer.Name); err != nil {
				return fmt.Errorf("test page could not be printed")
			}
			return nil
		}
	}
	return fmt.Errorf("printer was not found")
}
