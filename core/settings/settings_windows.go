//go:build windows

package settings

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// WindowsRegistry implements Registry over the Windows registry.
type WindowsRegistry struct{}

func registryRoot(root Root) registry.Key {
	if root == RootMachine {
		return registry.LOCAL_MACHINE
	}
	return registry.CURRENT_USER
}

func registryFlag(root Root) uint32 {
	if root == RootMachine {
		return registry.WOW64_64KEY
	}
	return 0
}

func (WindowsRegistry) open(root Root, path string, create bool) (registry.Key, error) {
	access := uint32(registry.QUERY_VALUE | registry.SET_VALUE)
	if create {
		access |= registry.CREATE_SUB_KEY
	}
	key, err := registry.OpenKey(registryRoot(root), path, access|registryFlag(root))
	if err != nil && create && errors.Is(err, registry.ErrNotExist) {
		created, _, createErr := registry.CreateKey(registryRoot(root), path, access|registryFlag(root))
		return created, createErr
	}
	return key, err
}

func (WindowsRegistry) StringValue(ctx context.Context, root Root, path, name string) (string, bool, error) {
	if err := ctx.Err(); err != nil {
		return "", false, err
	}
	key, err := registry.OpenKey(registryRoot(root), path, registry.QUERY_VALUE|registryFlag(root))
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return "", false, nil
		}
		return "", false, err
	}
	defer key.Close()
	value, _, err := key.GetStringValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return value, true, nil
}

func (WindowsRegistry) DWordValue(ctx context.Context, root Root, path, name string) (uint32, bool, error) {
	if err := ctx.Err(); err != nil {
		return 0, false, err
	}
	key, err := registry.OpenKey(registryRoot(root), path, registry.QUERY_VALUE|registryFlag(root))
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return 0, false, nil
		}
		return 0, false, err
	}
	defer key.Close()
	value, _, err := key.GetIntegerValue(name)
	if errors.Is(err, registry.ErrNotExist) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	return uint32(value), true, nil
}

func (WindowsRegistry) SetStringValue(ctx context.Context, root Root, path, name, value string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := (WindowsRegistry{}).open(root, path, true)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(name, value)
}

func (WindowsRegistry) SetDWordValue(ctx context.Context, root Root, path, name string, value uint32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := (WindowsRegistry{}).open(root, path, true)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetDWordValue(name, value)
}

func (WindowsRegistry) DeleteValue(ctx context.Context, root Root, path, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key, err := registry.OpenKey(registryRoot(root), path, registry.SET_VALUE|registryFlag(root))
	if err != nil {
		return nil
	}
	defer key.Close()
	if err := key.DeleteValue(name); err != nil && !errors.Is(err, registry.ErrNotExist) {
		return err
	}
	return nil
}

func (WindowsRegistry) KeyExists(ctx context.Context, root Root, path string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	key, err := registry.OpenKey(registryRoot(root), path, registry.QUERY_VALUE|registryFlag(root))
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	key.Close()
	return true, nil
}

func (WindowsRegistry) ValueExists(ctx context.Context, root Root, path, name string) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	key, err := registry.OpenKey(registryRoot(root), path, registry.QUERY_VALUE|registryFlag(root))
	if err != nil {
		if errors.Is(err, registry.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer key.Close()
	_, _, err = key.GetValue(name, nil)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	return err == nil, err
}

// WindowsRunner executes helper tools with a hidden window.
type WindowsRunner struct{}

func (WindowsRunner) Run(ctx context.Context, executable string, args ...string) (string, error) {
	runCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	cmd := exec.CommandContext(runCtx, executable, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	output, err := cmd.CombinedOutput()
	text := string(output)
	if len(text) > 65536 {
		text = text[len(text)-65536:]
	}
	if runCtx.Err() != nil {
		return text, runCtx.Err()
	}
	if err != nil {
		return text, err
	}
	return text, nil
}

var (
	advapi32               = windows.NewLazySystemDLL("advapi32.dll")
	kernel32               = windows.NewLazySystemDLL("kernel32.dll")
	slc                    = windows.NewLazySystemDLL("slc.dll")
	procOpenSCManagerW     = advapi32.NewProc("OpenSCManagerW")
	procOpenServiceW       = advapi32.NewProc("OpenServiceW")
	procQueryServiceStatus = advapi32.NewProc("QueryServiceStatusEx")
	procCloseServiceHandle = advapi32.NewProc("CloseServiceHandle")
	procSetComputerNameEx  = kernel32.NewProc("SetComputerNameExW")
	procSLIsGenuine        = slc.NewProc("SLIsWindowsGenuineLocal")
)

const (
	scManagerConnect                = 0x0001
	serviceQueryStatus              = 0x0004
	scStatusProcessInfo             = 0
	computerNamePhysicalDNSHostname = 5
)

// WindowsServices queries service state through the Service Control Manager.
type WindowsServices struct{}

type serviceStatusProcess struct {
	ServiceType             uint32
	CurrentState            uint32
	ControlsAccepted        uint32
	Win32ExitCode           uint32
	ServiceSpecificExitCode uint32
	CheckPoint              uint32
	WaitHint                uint32
	ProcessId               uint32
	ServiceFlags            uint32
}

func (WindowsServices) State(ctx context.Context, name string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	manager, _, _ := procOpenSCManagerW.Call(0, 0, scManagerConnect)
	if manager == 0 {
		return "", fmt.Errorf("service manager is unavailable")
	}
	defer procCloseServiceHandle.Call(manager)
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return "", fmt.Errorf("invalid service name")
	}
	service, _, _ := procOpenServiceW.Call(manager, uintptr(unsafe.Pointer(namePtr)), serviceQueryStatus)
	if service == 0 {
		return "", fmt.Errorf("service is unavailable")
	}
	defer procCloseServiceHandle.Call(service)
	status := serviceStatusProcess{}
	var needed uint32
	result, _, _ := procQueryServiceStatus.Call(service, scStatusProcessInfo, uintptr(unsafe.Pointer(&status)), uintptr(unsafe.Sizeof(status)), uintptr(unsafe.Pointer(&needed)))
	if result == 0 {
		return "", fmt.Errorf("service status is unavailable")
	}
	states := map[uint32]string{1: "stopped", 2: "start-pending", 3: "stop-pending", 4: "running", 5: "continue-pending", 6: "pause-pending", 7: "paused"}
	if state, ok := states[status.CurrentState]; ok {
		return state, nil
	}
	return "unknown", nil
}

// WindowsComputerName renames the computer through SetComputerNameExW.
type WindowsComputerName struct{}

func (WindowsComputerName) SetComputerName(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return fmt.Errorf("invalid computer name")
	}
	result, _, _ := procSetComputerNameEx.Call(computerNamePhysicalDNSHostname, uintptr(unsafe.Pointer(namePtr)))
	if result == 0 {
		return fmt.Errorf("computer name could not be changed")
	}
	return nil
}

// WindowsActivation reports whether Windows is genuine, read-only.
type WindowsActivation struct{}

func (WindowsActivation) Status(ctx context.Context) (Activation, error) {
	if err := ctx.Err(); err != nil {
		return Activation{}, err
	}
	var genuine int32
	result, _, _ := procSLIsGenuine.Call(uintptr(unsafe.Pointer(&genuine)))
	if int32(uint32(result)) < 0 {
		return Activation{Status: "unknown", Detail: "SLIsWindowsGenuineLocal недоступен"}, nil
	}
	info := Activation{Genuine: genuine != 0, Status: "not-genuine", Detail: "Используйте штатные средства активации Windows (slmgr.vbs /dlv)."}
	if genuine != 0 {
		info.Status = "genuine"
		info.Detail = "Windows подтверждает подлинность. WinDeploy не изменяет лицензирование."
	}
	return info, nil
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
