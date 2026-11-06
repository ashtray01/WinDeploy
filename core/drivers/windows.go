//go:build windows

package drivers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"windeploy/core/inventory"
)

var (
	setupapi                    = windows.NewLazySystemDLL("setupapi.dll")
	cfgmgr32                    = windows.NewLazySystemDLL("cfgmgr32.dll")
	procSetupDiGetClassDevs     = setupapi.NewProc("SetupDiGetClassDevsW")
	procSetupDiDestroyList      = setupapi.NewProc("SetupDiDestroyDeviceInfoList")
	procSetupDiEnumDeviceInfo   = setupapi.NewProc("SetupDiEnumDeviceInfo")
	procSetupDiGetDeviceRegProp = setupapi.NewProc("SetupDiGetDeviceRegistryPropertyW")
	procSetupDiGetInstanceID    = setupapi.NewProc("SetupDiGetDeviceInstanceIdW")
	procSetupVerifyInfFile      = setupapi.NewProc("SetupVerifyInfFileW")
	procCMGetDevNodeStatus      = cfgmgr32.NewProc("CM_Get_DevNode_Status")
)

const (
	digcfPresent      = 0x00000002
	digcfAllClasses   = 0x00000004
	spdrpDeviceDesc   = 0x00000000
	spdrpHardwareID   = 0x00000001
	spdrpClass        = 0x00000007
	spdrpDriver       = 0x00000009
	spdrpMfg          = 0x0000000B
	spdrpFriendlyName = 0x0000000C
	dnHasProblem      = 0x00000400
	errorNoMoreItems  = 259
	invalidHandle     = ^uintptr(0)
)

type spDevInfoData struct {
	Size      uint32
	ClassGUID windows.GUID
	DevInst   uint32
	Reserved  uintptr
}

// WindowsDevices enumerates present PnP devices through SetupAPI. It is read-only
// and needs no administrator rights.
type WindowsDevices struct{}

func (WindowsDevices) Devices(ctx context.Context) ([]Device, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	handle, _, _ := procSetupDiGetClassDevs.Call(0, 0, 0, digcfPresent|digcfAllClasses)
	if handle == invalidHandle || handle == 0 {
		return nil, fmt.Errorf("device enumeration could not start")
	}
	defer procSetupDiDestroyList.Call(handle)
	devices := []Device{}
	for index := uint32(0); index < 5000; index++ {
		if err := ctx.Err(); err != nil {
			return devices, err
		}
		info := spDevInfoData{Size: uint32(unsafe.Sizeof(spDevInfoData{}))}
		result, _, callErr := procSetupDiEnumDeviceInfo.Call(handle, uintptr(index), uintptr(unsafe.Pointer(&info)))
		if result == 0 {
			if errno, ok := callErr.(syscall.Errno); ok && errno == errorNoMoreItems {
				break
			}
			if index == 0 {
				return nil, fmt.Errorf("device enumeration failed")
			}
			break
		}
		device := readDevice(handle, &info)
		if device.InstanceID == "" {
			continue
		}
		devices = append(devices, device)
	}
	sort.SliceStable(devices, func(i, j int) bool {
		left := strings.ToLower(devices[i].Class) + "\x00" + strings.ToLower(devices[i].Name)
		right := strings.ToLower(devices[j].Class) + "\x00" + strings.ToLower(devices[j].Name)
		if left == right {
			return devices[i].InstanceID < devices[j].InstanceID
		}
		return left < right
	})
	return devices, nil
}

func readDevice(handle uintptr, info *spDevInfoData) Device {
	device := Device{HardwareIDs: []string{}, Status: "unknown"}
	device.InstanceID = deviceInstanceID(handle, info)
	device.Name = propertyString(handle, info, spdrpDeviceDesc)
	if friendly := propertyString(handle, info, spdrpFriendlyName); friendly != "" {
		device.Name = friendly
	}
	device.Class = propertyString(handle, info, spdrpClass)
	device.Manufacturer = propertyString(handle, info, spdrpMfg)
	device.HardwareIDs = multiStringProperty(handle, info, spdrpHardwareID)
	driverKey := propertyString(handle, info, spdrpDriver)
	if driverKey != "" {
		device.DriverVersion, device.DriverProvider = driverInfo(driverKey)
	}
	var status, problem uint32
	result, _, _ := procCMGetDevNodeStatus.Call(uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Pointer(&problem)), uintptr(info.DevInst), 0)
	if result == 0 {
		device.ProblemCode = problem
		if status&dnHasProblem != 0 {
			device.Status = "problem"
		} else {
			device.Status = "ok"
		}
	}
	device.Unknown = device.ProblemCode == 28 || (device.Status == "problem" && device.DriverVersion == "" && device.Class == "")
	return device
}

func deviceInstanceID(handle uintptr, info *spDevInfoData) string {
	size := uint32(1024)
	buffer := make([]uint16, size)
	result, _, _ := procSetupDiGetInstanceID.Call(handle, uintptr(unsafe.Pointer(info)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(size), 0)
	if result == 0 {
		return ""
	}
	return windows.UTF16ToString(buffer)
}

func propertyBytes(handle uintptr, info *spDevInfoData, property uint32) ([]byte, uint32, bool) {
	var regType uint32
	size := uint32(2048)
	for attempt := 0; attempt < 2; attempt++ {
		buffer := make([]byte, size)
		var required uint32
		result, _, _ := procSetupDiGetDeviceRegProp.Call(handle, uintptr(unsafe.Pointer(info)), uintptr(property), uintptr(unsafe.Pointer(&regType)), uintptr(unsafe.Pointer(&buffer[0])), uintptr(size), uintptr(unsafe.Pointer(&required)))
		if result != 0 {
			if required > 0 && required < size {
				return buffer[:required], regType, true
			}
			return buffer, regType, true
		}
		if required > size && required <= 1<<20 {
			size = required
			continue
		}
		return nil, regType, false
	}
	return nil, regType, false
}

func propertyString(handle uintptr, info *spDevInfoData, property uint32) string {
	bytes, regType, ok := propertyBytes(handle, info, property)
	if !ok || regType != registry.SZ || len(bytes) < 2 {
		return ""
	}
	for i := 0; i+1 < len(bytes); i += 2 {
		if bytes[i] == 0 && bytes[i+1] == 0 {
			bytes = bytes[:i]
			break
		}
	}
	return decodeUTF16(bytes)
}

func multiStringProperty(handle uintptr, info *spDevInfoData, property uint32) []string {
	bytes, regType, ok := propertyBytes(handle, info, property)
	if !ok || regType != registry.MULTI_SZ || len(bytes) < 4 {
		return []string{}
	}
	units := make([]uint16, 0, len(bytes)/2)
	for i := 0; i+1 < len(bytes); i += 2 {
		units = append(units, uint16(bytes[i])|uint16(bytes[i+1])<<8)
	}
	values := []string{}
	current := make([]uint16, 0, 128)
	for _, unit := range units {
		if unit == 0 {
			if len(current) == 0 {
				break
			}
			values = append(values, string(utf16.Decode(current)))
			current = current[:0]
			continue
		}
		current = append(current, unit)
	}
	return values
}

func driverInfo(driverKey string) (string, string) {
	path := `SYSTEM\CurrentControlSet\Control\Class\` + driverKey
	key, err := registry.OpenKey(registry.LOCAL_MACHINE, path, registry.QUERY_VALUE)
	if err != nil {
		return "", ""
	}
	defer key.Close()
	version, _, _ := key.GetStringValue("DriverVersion")
	provider, _, _ := key.GetStringValue("ProviderName")
	return strings.TrimSpace(version), strings.TrimSpace(provider)
}

// WindowsINFVerifier checks INF/catalog signatures through SetupVerifyInfFileW,
// which also enforces the current platform and architecture.
type WindowsINFVerifier struct{}

type spInfSignerInfo struct {
	Size          uint32
	CatalogFile   [260]uint16
	DigitalSigner [260]uint16
	SignerVersion [260]uint16
}

func (WindowsINFVerifier) VerifyINF(ctx context.Context, infPath string) (Signature, error) {
	if err := ctx.Err(); err != nil {
		return Signature{}, err
	}
	pathPtr, err := windows.UTF16PtrFromString(infPath)
	if err != nil {
		return Signature{}, fmt.Errorf("invalid INF path")
	}
	info := spInfSignerInfo{Size: uint32(unsafe.Sizeof(spInfSignerInfo{}))}
	result, _, _ := procSetupVerifyInfFile.Call(uintptr(unsafe.Pointer(pathPtr)), 0, uintptr(unsafe.Pointer(&info)))
	return Signature{
		Verified: result != 0,
		Signer:   strings.TrimSpace(windows.UTF16ToString(info.DigitalSigner[:])),
		Catalog:  strings.TrimSpace(windows.UTF16ToString(info.CatalogFile[:])),
	}, nil
}

// WindowsInstaller installs INF packages through pnputil. Windows 7 uses the
// legacy syntax; Windows 10/11 use /add-driver /install.
type WindowsInstaller struct {
	Runner Runner
	System inventory.SystemInfo
}

func (i WindowsInstaller) Install(ctx context.Context, pkg DriverPackage) (InstallOutcome, error) {
	if i.Runner == nil {
		return InstallOutcome{}, fmt.Errorf("process runner is required")
	}
	executable := findPnputil()
	if executable == "" {
		return InstallOutcome{}, fmt.Errorf("pnputil is unavailable")
	}
	args := []string{"/add-driver", pkg.INFPath, "/install"}
	if i.System.Windows.Major == 6 && i.System.Windows.Minor == 1 {
		args = []string{"-i", "-a", pkg.INFPath}
	}
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	execution, err := i.Runner.Run(runCtx, Command{Path: executable, Args: args})
	if err != nil {
		return InstallOutcome{ExitCode: execution.ExitCode}, fmt.Errorf("pnputil could not be started")
	}
	outcome := InstallOutcome{ExitCode: execution.ExitCode}
	if execution.ExitCode == 0 || execution.ExitCode == 3010 || execution.ExitCode == 1641 {
		outcome.RebootRequired = execution.ExitCode != 0
		return outcome, nil
	}
	return outcome, fmt.Errorf("pnputil failed")
}

func findPnputil() string {
	root := os.Getenv("SystemRoot")
	candidates := []string{}
	if root != "" {
		candidates = append(candidates,
			filepath.Join(root, "System32", "pnputil.exe"),
			filepath.Join(root, "Sysnative", "pnputil.exe"),
		)
	}
	if path, err := exec.LookPath("pnputil.exe"); err == nil {
		candidates = append(candidates, path)
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	return ""
}

// ScanDevices triggers a PnP scan so Windows can match available drivers,
// including drivers distributed through Windows Update policy. It does not install
// anything itself and requires elevation.
func (i WindowsInstaller) ScanDevices(ctx context.Context) error {
	if i.Runner == nil {
		return fmt.Errorf("process runner is required")
	}
	if i.System.Windows.Major == 6 && i.System.Windows.Minor == 1 {
		return ErrUnsupported
	}
	executable := findPnputil()
	if executable == "" {
		return fmt.Errorf("pnputil is unavailable")
	}
	runCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	execution, err := i.Runner.Run(runCtx, Command{Path: executable, Args: []string{"/scan-devices"}})
	if err != nil {
		return fmt.Errorf("driver rescan could not be started")
	}
	if execution.ExitCode != 0 {
		return fmt.Errorf("driver rescan failed")
	}
	return nil
}

// WindowsRunner executes helper processes with a hidden console window.
type WindowsRunner struct{}

func (WindowsRunner) Run(ctx context.Context, command Command) (Execution, error) {
	if strings.TrimSpace(command.Path) == "" {
		return Execution{}, fmt.Errorf("executable path is required")
	}
	cmd := exec.CommandContext(ctx, command.Path, command.Args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	output, err := cmd.CombinedOutput()
	text := string(output)
	if len(text) > 8192 {
		text = text[len(text)-8192:]
	}
	if ctx.Err() != nil {
		return Execution{ExitCode: -1, Output: sanitize(text)}, ctx.Err()
	}
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return Execution{ExitCode: exitErr.ExitCode(), Output: sanitize(text)}, nil
		}
		return Execution{}, fmt.Errorf("process could not be started")
	}
	return Execution{ExitCode: 0, Output: sanitize(text)}, nil
}

func sanitize(value string) string {
	var builder strings.Builder
	for _, r := range value {
		if r < 32 && r != '\n' && r != '\t' {
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

// WindowsElevation reports the current token elevation.
type WindowsElevation struct{}

func (WindowsElevation) Elevated(ctx context.Context) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	token, err := windows.OpenCurrentProcessToken()
	if err != nil {
		return false, err
	}
	defer token.Close()
	return token.IsElevated(), nil
}
