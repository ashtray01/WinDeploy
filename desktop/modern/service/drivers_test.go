package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"windeploy/core/printers"
)

func TestGetDevicesReadsRealHardware(t *testing.T) {
	service := &Service{}
	devices, err := service.GetDevices()
	if err != nil {
		t.Fatalf("device enumeration failed: %v", err)
	}
	if len(devices) == 0 {
		t.Fatal("no devices returned")
	}
	for _, device := range devices {
		if device.InstanceID == "" {
			t.Fatal("device without instance ID")
		}
	}
}

func TestScanDriverCatalogRejectsMissingDirectory(t *testing.T) {
	service := &Service{}
	if _, err := service.ScanDriverCatalog(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing directory was accepted")
	}
}

func TestInstallDriverRejectsUnsignedINF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unsigned.inf")
	if err := os.WriteFile(path, []byte("[Version]\nSignature=\"$Windows NT$\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := &Service{}
	_, err := service.InstallDriver(DriverInstallRequest{INFPath: path, HardwareID: `PCI\VEN_1234&DEV_5678`, DryRun: true})
	if err == nil || !strings.Contains(err.Error(), "Подпись") {
		t.Fatalf("unsigned INF accepted: %v", err)
	}
}

func TestPrintersService(t *testing.T) {
	service := &Service{}
	if _, err := service.GetPrinters(); err != nil {
		t.Fatalf("printer enumeration failed: %v", err)
	}
	models := service.GetKyocera2460Models()
	if len(models) < 2 {
		t.Fatalf("unexpected Kyocera models: %v", models)
	}
}

func TestAddPrinterDryRunValidates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "printer.inf")
	if err := os.WriteFile(path, []byte("[Version]\nSignature=\"$Windows NT$\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	service := &Service{}
	result, err := service.AddPrinter(printers.AddRequest{Name: "Test Printer", INFPath: path, Model: "Generic / Text Only", IPAddress: "192.168.1.5", DryRun: true})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if result.Installed || !strings.Contains(result.Message, "Проверка") {
		t.Fatalf("unexpected dry-run result: %+v", result)
	}
	if _, err := service.AddPrinter(printers.AddRequest{Name: "Bad", INFPath: path, Model: "x", IPAddress: "not-an-ip"}); err == nil {
		t.Fatal("invalid IP accepted")
	}
}
