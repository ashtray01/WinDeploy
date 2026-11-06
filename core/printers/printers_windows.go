//go:build windows

package printers

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	winspool              = windows.NewLazySystemDLL("winspool.drv")
	gdi32                 = windows.NewLazySystemDLL("gdi32.dll")
	procEnumPrinters      = winspool.NewProc("EnumPrintersW")
	procGetDefaultPrinter = winspool.NewProc("GetDefaultPrinterW")
	procSetDefaultPrinter = winspool.NewProc("SetDefaultPrinterW")
	procCreateDC          = gdi32.NewProc("CreateDCW")
	procDeleteDC          = gdi32.NewProc("DeleteDC")
	procStartDoc          = gdi32.NewProc("StartDocW")
	procStartPage         = gdi32.NewProc("StartPage")
	procEndPage           = gdi32.NewProc("EndPage")
	procEndDoc            = gdi32.NewProc("EndDoc")
	procTextOut           = gdi32.NewProc("TextOutW")
)

const (
	printerEnumLocal        = 0x00000002
	printerEnumConnections  = 0x00000004
	printerAttributeOffline = 0x00000400
	printerStatusOffline    = 0x00000080
	printerStatusError      = 0x00000002
)

type printerInfo2 struct {
	ServerName         *uint16
	PrinterName        *uint16
	ShareName          *uint16
	PortName           *uint16
	DriverName         *uint16
	Comment            *uint16
	Location           *uint16
	DevMode            uintptr
	SepFile            *uint16
	PrintProcessor     *uint16
	Datatype           *uint16
	Parameters         *uint16
	SecurityDescriptor uintptr
	Attributes         uint32
	Priority           uint32
	DefaultPriority    uint32
	StartTime          uint32
	UntilTime          uint32
	Status             uint32
	Jobs               uint32
	AveragePPM         uint32
}

// WindowsPrinters enumerates local and connected print queues read-only.
type WindowsPrinters struct{}

func (WindowsPrinters) Printers(ctx context.Context) ([]Printer, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var needed, returned uint32
	procEnumPrinters.Call(printerEnumLocal|printerEnumConnections, 0, 2, 0, 0, uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if needed == 0 || needed > 32<<20 {
		return []Printer{}, nil
	}
	buffer := make([]byte, needed)
	result, _, _ := procEnumPrinters.Call(printerEnumLocal|printerEnumConnections, 0, 2, uintptr(unsafe.Pointer(&buffer[0])), uintptr(needed), uintptr(unsafe.Pointer(&needed)), uintptr(unsafe.Pointer(&returned)))
	if result == 0 {
		return nil, fmt.Errorf("printers could not be enumerated")
	}
	if returned > 4096 {
		return nil, fmt.Errorf("too many printers")
	}
	defaultName, _ := (WindowsDefaults{}).Default(ctx)
	printers := make([]Printer, 0, returned)
	if returned > 0 {
		entries := unsafe.Slice((*printerInfo2)(unsafe.Pointer(&buffer[0])), int(returned))
		for _, entry := range entries {
			printer := Printer{
				Name:       windows.UTF16PtrToString(entry.PrinterName),
				Driver:     windows.UTF16PtrToString(entry.DriverName),
				Port:       windows.UTF16PtrToString(entry.PortName),
				Server:     windows.UTF16PtrToString(entry.ServerName),
				ShareName:  windows.UTF16PtrToString(entry.ShareName),
				Default:    defaultName != "" && strings.EqualFold(defaultName, windows.UTF16PtrToString(entry.PrinterName)),
				StatusCode: entry.Status,
			}
			printer.Shared = printer.ShareName != ""
			switch {
			case entry.Attributes&printerAttributeOffline != 0, entry.Status&printerStatusOffline != 0:
				printer.Status = "offline"
			case entry.Status&printerStatusError != 0:
				printer.Status = "error"
			case entry.Status != 0:
				printer.Status = "warning"
			default:
				printer.Status = "ok"
			}
			if printer.Name != "" {
				printers = append(printers, printer)
			}
		}
	}
	sort.SliceStable(printers, func(i, j int) bool { return strings.ToLower(printers[i].Name) < strings.ToLower(printers[j].Name) })
	return printers, nil
}

// WindowsDefaults manages the per-user default printer.
type WindowsDefaults struct{}

func (WindowsDefaults) Default(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	var needed uint32
	procGetDefaultPrinter.Call(0, uintptr(unsafe.Pointer(&needed)))
	if needed == 0 || needed > 4096 {
		return "", nil
	}
	buffer := make([]uint16, needed)
	result, _, _ := procGetDefaultPrinter.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&needed)))
	if result == 0 {
		return "", nil
	}
	return windows.UTF16ToString(buffer), nil
}

func (WindowsDefaults) SetDefault(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return fmt.Errorf("invalid printer name")
	}
	result, _, _ := procSetDefaultPrinter.Call(uintptr(unsafe.Pointer(namePtr)))
	if result == 0 {
		return fmt.Errorf("default printer could not be changed")
	}
	return nil
}

// WindowsInstaller installs a print queue through printui.dll. The INF provides
// the driver; the IP address creates a standard TCP/IP port.
type WindowsInstaller struct{}

func (WindowsInstaller) AddPrinter(ctx context.Context, request AddRequest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	port := request.IPAddress
	args := []string{
		"printui.dll,PrintUIEntry", "/if",
		"/b", request.Name,
		"/f", request.INFPath,
		"/r", "IP_" + port,
		"/m", request.Model,
		"/q",
	}
	if request.Share && request.ShareName != "" {
		args = append(args, "/z", request.ShareName)
	}
	runCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(runCtx, "rundll32.exe", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	if err := cmd.Run(); err != nil {
		if runCtx.Err() != nil {
			return fmt.Errorf("printer installation timed out")
		}
		return fmt.Errorf("printer installation failed")
	}
	return nil
}

// WindowsTestPrinter prints a minimal real page through the GDI print path.
type WindowsTestPrinter struct{}

type docInfoW struct {
	Size     int32
	DocName  *uint16
	Output   *uint16
	Datatype *uint16
	Type     uint32
}

func (WindowsTestPrinter) PrintTestPage(ctx context.Context, name string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	namePtr, err := windows.UTF16PtrFromString(name)
	if err != nil {
		return fmt.Errorf("invalid printer name")
	}
	driverPtr, _ := windows.UTF16PtrFromString("WINSPOOL")
	dc, _, _ := procCreateDC.Call(uintptr(unsafe.Pointer(driverPtr)), uintptr(unsafe.Pointer(namePtr)), 0, 0)
	if dc == 0 {
		return fmt.Errorf("printer device context is unavailable")
	}
	defer procDeleteDC.Call(dc)
	docName, _ := windows.UTF16PtrFromString("WinDeploy — тестовая страница")
	info := docInfoW{Size: int32(unsafe.Sizeof(docInfoW{})), DocName: docName}
	if result, _, _ := procStartDoc.Call(dc, uintptr(unsafe.Pointer(&info))); int32(result) <= 0 {
		return fmt.Errorf("print job could not be created")
	}
	defer procEndDoc.Call(dc)
	if result, _, _ := procStartPage.Call(dc); int32(result) <= 0 {
		return fmt.Errorf("print page could not be started")
	}
	lines := []string{
		"WinDeploy — проверка принтера",
		"Принтер: " + name,
		"Дата: " + time.Now().Format("02.01.2006 15:04"),
	}
	for index, line := range lines {
		text, textErr := windows.UTF16FromString(line)
		if textErr != nil {
			continue
		}
		procTextOut.Call(dc, 100, uintptr(200+index*40), uintptr(unsafe.Pointer(&text[0])), uintptr(len(text)-1))
	}
	procEndPage.Call(dc)
	return nil
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
