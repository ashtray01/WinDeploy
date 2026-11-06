// Package drivers inspects PnP devices and installs reviewed INF driver packages.
// It never hides a signature failure and never installs without elevation and an
// explicit request.
package drivers

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"windeploy/core/inventory"
)

// Device is a present PnP device. Driver fields may be empty when Windows has no
// installed driver (for example an unknown device with problem code 28).
type Device struct {
	InstanceID     string   `json:"instance_id"`
	Name           string   `json:"name"`
	Class          string   `json:"class,omitempty"`
	Manufacturer   string   `json:"manufacturer,omitempty"`
	HardwareIDs    []string `json:"hardware_ids"`
	DriverVersion  string   `json:"driver_version,omitempty"`
	DriverProvider string   `json:"driver_provider,omitempty"`
	Status         string   `json:"status"` // ok, problem, unknown
	ProblemCode    uint32   `json:"problem_code,omitempty"`
	Unknown        bool     `json:"unknown"`
}

// DeviceProvider enumerates present devices read-only.
type DeviceProvider interface {
	Devices(context.Context) ([]Device, error)
}

// Signature is the result of checking an INF/catalog signature.
type Signature struct {
	Verified bool   `json:"verified"`
	Signer   string `json:"signer,omitempty"`
	Catalog  string `json:"catalog,omitempty"`
}

// INFVerifier verifies that Windows accepts the INF signature for this platform.
type INFVerifier interface {
	VerifyINF(context.Context, string) (Signature, error)
}

// DriverPackage describes the INF selected for installation.
type DriverPackage struct {
	INFPath    string `json:"inf_path"`
	HardwareID string `json:"hardware_id"`
	Version    string `json:"version,omitempty"`
	Provider   string `json:"provider,omitempty"`
}

// InstallOutcome is the raw installer result.
type InstallOutcome struct {
	ExitCode       int  `json:"exit_code"`
	RebootRequired bool `json:"reboot_required"`
}

// Installer installs one driver package. Implementations must require elevation
// and must not silently ignore signed-driver policy errors.
type Installer interface {
	Install(context.Context, DriverPackage) (InstallOutcome, error)
}

// Command is one local process invocation without a shell.
type Command struct {
	Path string
	Args []string
}

// Execution is a bounded process outcome.
type Execution struct {
	ExitCode int
	Output   string
}

// Runner executes local processes.
type Runner interface {
	Run(context.Context, Command) (Execution, error)
}

// Elevation reports administrator rights.
type Elevation interface {
	Elevated(context.Context) (bool, error)
}

// ErrElevationRequired is returned before any install without an elevated token.
var ErrElevationRequired = errors.New("administrator privileges are required")

// ErrUnsignedINF is returned when Windows rejects the INF/catalog signature.
var ErrUnsignedINF = errors.New("driver package signature is not trusted")

// ErrUnsupported is returned when the platform cannot perform the operation.
var ErrUnsupported = errors.New("driver operation is unsupported on this platform")

// Result is a safe installation outcome.
type Result struct {
	INFPath        string    `json:"inf_path"`
	HardwareID     string    `json:"hardware_id"`
	Signature      Signature `json:"signature"`
	Installed      bool      `json:"installed"`
	RebootRequired bool      `json:"reboot_required"`
	ExitCode       int       `json:"exit_code"`
	Device         *Device   `json:"device,omitempty"`
	Message        string    `json:"message"`
}

// InstallRequest controls one driver installation.
type InstallRequest struct {
	INFPath    string
	HardwareID string
	DryRun     bool
}

// Manager coordinates verification, installation and post-check.
type Manager struct {
	Devices   DeviceProvider
	Installer Installer
	Verifier  INFVerifier
	Elevation Elevation
	System    inventory.SystemInfo
}

// Install verifies the INF signature, checks elevation and installs the package.
// Post-install verification looks for the hardware ID with no problem code.
func (m *Manager) Install(ctx context.Context, request InstallRequest) (Result, error) {
	result := Result{INFPath: strings.TrimSpace(request.INFPath), HardwareID: strings.TrimSpace(request.HardwareID)}
	if result.INFPath == "" || result.HardwareID == "" {
		return result, fmt.Errorf("INF path and hardware ID are required")
	}
	absolute, err := filepath.Abs(result.INFPath)
	if err != nil {
		return result, fmt.Errorf("invalid INF path")
	}
	result.INFPath = absolute
	info, err := os.Stat(absolute)
	if err != nil || !info.Mode().IsRegular() || !strings.EqualFold(filepath.Ext(absolute), ".inf") {
		return result, fmt.Errorf("INF file is unavailable")
	}
	if m.Verifier == nil {
		return result, fmt.Errorf("INF verifier is required")
	}
	signature, err := m.Verifier.VerifyINF(ctx, absolute)
	result.Signature = signature
	if err != nil || !signature.Verified {
		return result, ErrUnsignedINF
	}
	if m.Installer == nil {
		return result, fmt.Errorf("driver installer is required")
	}
	if !request.DryRun {
		if m.Elevation == nil {
			return result, ErrElevationRequired
		}
		elevated, elevateErr := m.Elevation.Elevated(ctx)
		if elevateErr != nil {
			return result, ErrElevationRequired
		}
		if !elevated {
			return result, ErrElevationRequired
		}
	}
	pkg := DriverPackage{INFPath: absolute, HardwareID: result.HardwareID}
	if request.DryRun {
		result.Message = "Проверка выполнена: подпись принята, установка не запускалась."
		return result, nil
	}
	outcome, err := m.Installer.Install(ctx, pkg)
	result.ExitCode = outcome.ExitCode
	result.RebootRequired = outcome.RebootRequired
	if err != nil {
		return result, fmt.Errorf("driver installation failed")
	}
	if m.Devices != nil {
		devices, listErr := m.Devices.Devices(ctx)
		if listErr == nil {
			for i := range devices {
				if matchesHardwareID(devices[i], result.HardwareID) {
					device := devices[i]
					result.Device = &device
					break
				}
			}
		}
	}
	if result.Device != nil {
		result.Installed = result.Device.Status == "ok"
	}
	if result.RebootRequired {
		result.Message = "Драйвер установлен. Требуется перезагрузка для завершения."
		result.Installed = true
		return result, nil
	}
	if !result.Installed {
		return result, fmt.Errorf("installed driver was not confirmed on a matching device")
	}
	result.Message = "Драйвер установлен и подтверждён."
	return result, nil
}

func matchesHardwareID(device Device, hardwareID string) bool {
	for _, id := range device.HardwareIDs {
		if strings.EqualFold(id, hardwareID) {
			return true
		}
	}
	return false
}
