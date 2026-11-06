package drivers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

const sampleINF = `; Example driver package
[Version]
Signature="$Windows NT$"
Class=Net
Provider=%Mfg%
DriverVer=05/10/2024,1.2.3.4

[Manufacturer]
%Mfg%=Models,NTamd64

[Models]
%Device%=InstallSection, PCI\VEN_1234&DEV_5678, USB\VID_1234&PID_5678

[InstallSection]
AddReg=Some.Reg

[Strings]
Mfg="Example Corp"
Device="Example Device"
`

func TestParseINFExpandsStringsAndHardwareIDs(t *testing.T) {
	info, err := ParseINF(strings.NewReader(sampleINF))
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if info.Provider != "Example Corp" || info.Version != "1.2.3.4" || info.Class != "Net" {
		t.Fatalf("unexpected metadata: %+v", info)
	}
	if len(info.HardwareIDs) != 2 {
		t.Fatalf("hardware IDs = %v", info.HardwareIDs)
	}
	if info.HardwareIDs[0] != `PCI\VEN_1234&DEV_5678` || info.HardwareIDs[1] != `USB\VID_1234&PID_5678` {
		t.Fatalf("unexpected hardware IDs: %v", info.HardwareIDs)
	}
}

func TestParseINFUTF16(t *testing.T) {
	units := utf16.Encode([]rune(sampleINF))
	data := []byte{0xFF, 0xFE}
	for _, unit := range units {
		data = append(data, byte(unit), byte(unit>>8))
	}
	info, err := ParseINF(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("UTF-16 parse failed: %v", err)
	}
	if info.Provider != "Example Corp" || len(info.HardwareIDs) != 2 {
		t.Fatalf("unexpected UTF-16 metadata: %+v", info)
	}
}

func TestParseINFRejectsGarbage(t *testing.T) {
	if _, err := ParseINF(strings.NewReader("not an inf file")); err == nil {
		t.Fatal("garbage was accepted as a driver package")
	}
}

func TestScanDirectoryAndFindHardwareID(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "net", "adapter")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "example.inf"), []byte(sampleINF), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "readme.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatal(err)
	}
	packages, err := ScanDirectory(context.Background(), root, 100)
	if err != nil || len(packages) != 1 {
		t.Fatalf("scan result: %+v, %v", packages, err)
	}
	matches, err := FindForHardwareID(context.Background(), root, `pci\ven_1234&dev_5678`)
	if err != nil || len(matches) != 1 {
		t.Fatalf("hardware match: %+v, %v", matches, err)
	}
	if len(matches[0].HardwareIDs) != 2 {
		t.Fatalf("unexpected matches: %+v", matches[0])
	}
	none, err := FindForHardwareID(context.Background(), root, `PCI\VEN_9999&DEV_0000`)
	if err != nil || len(none) != 0 {
		t.Fatalf("unexpected non-match: %+v, %v", none, err)
	}
}

type fakeDevices struct {
	items  []Device
	calls  int
	err    error
	onList func()
}

func (f *fakeDevices) Devices(context.Context) ([]Device, error) {
	f.calls++
	if f.onList != nil {
		f.onList()
	}
	return append([]Device{}, f.items...), f.err
}

type fakeInstaller struct {
	calls     int
	pkg       DriverPackage
	outcome   InstallOutcome
	err       error
	onInstall func()
}

func (f *fakeInstaller) Install(_ context.Context, pkg DriverPackage) (InstallOutcome, error) {
	f.calls++
	f.pkg = pkg
	if f.onInstall != nil {
		f.onInstall()
	}
	return f.outcome, f.err
}

type fakeVerifier struct {
	signature Signature
	err       error
}

func (f fakeVerifier) VerifyINF(context.Context, string) (Signature, error) {
	return f.signature, f.err
}

type fakeElevation bool

func (e fakeElevation) Elevated(context.Context) (bool, error) { return bool(e), nil }

func writeTestINF(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "example.inf")
	if err := os.WriteFile(path, []byte(sampleINF), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestManagerInstallsVerifiedDriver(t *testing.T) {
	infPath := writeTestINF(t)
	devices := &fakeDevices{}
	installer := &fakeInstaller{onInstall: func() {
		devices.items = append(devices.items, Device{InstanceID: `PCI\VEN_1234&DEV_5678\1`, HardwareIDs: []string{`PCI\VEN_1234&DEV_5678`}, Status: "ok", DriverVersion: "1.2.3.4"})
	}}
	manager := &Manager{Devices: devices, Installer: installer, Verifier: fakeVerifier{signature: Signature{Verified: true, Signer: "Example Corp"}}, Elevation: fakeElevation(true)}
	result, err := manager.Install(context.Background(), InstallRequest{INFPath: infPath, HardwareID: `PCI\VEN_1234&DEV_5678`})
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}
	if !result.Installed || result.Device == nil || result.Device.DriverVersion != "1.2.3.4" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if installer.calls != 1 || !strings.HasSuffix(strings.ToLower(installer.pkg.INFPath), "example.inf") {
		t.Fatalf("unexpected installer call: %+v", installer)
	}
}

func TestManagerRejectsUnsignedAndNonElevated(t *testing.T) {
	infPath := writeTestINF(t)
	installer := &fakeInstaller{}
	manager := &Manager{Devices: &fakeDevices{}, Installer: installer, Verifier: fakeVerifier{signature: Signature{Verified: false}}, Elevation: fakeElevation(true)}
	if _, err := manager.Install(context.Background(), InstallRequest{INFPath: infPath, HardwareID: `PCI\VEN_1234&DEV_5678`}); !errors.Is(err, ErrUnsignedINF) {
		t.Fatalf("expected unsigned error, got %v", err)
	}
	if installer.calls != 0 {
		t.Fatal("unsigned package must not be installed")
	}
	manager.Verifier = fakeVerifier{signature: Signature{Verified: true}}
	manager.Elevation = fakeElevation(false)
	if _, err := manager.Install(context.Background(), InstallRequest{INFPath: infPath, HardwareID: `PCI\VEN_1234&DEV_5678`}); !errors.Is(err, ErrElevationRequired) {
		t.Fatalf("expected elevation error, got %v", err)
	}
	if installer.calls != 0 {
		t.Fatal("install must not run without elevation")
	}
}

func TestManagerDryRunDoesNotInstall(t *testing.T) {
	infPath := writeTestINF(t)
	installer := &fakeInstaller{}
	manager := &Manager{Devices: &fakeDevices{}, Installer: installer, Verifier: fakeVerifier{signature: Signature{Verified: true}}, Elevation: fakeElevation(false)}
	result, err := manager.Install(context.Background(), InstallRequest{INFPath: infPath, HardwareID: `PCI\VEN_1234&DEV_5678`, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if installer.calls != 0 || result.Installed {
		t.Fatalf("dry-run changed state: %+v", result)
	}
}

func TestManagerFailsWhenDeviceIsNotConfirmed(t *testing.T) {
	infPath := writeTestINF(t)
	installer := &fakeInstaller{outcome: InstallOutcome{ExitCode: 0}}
	manager := &Manager{Devices: &fakeDevices{}, Installer: installer, Verifier: fakeVerifier{signature: Signature{Verified: true}}, Elevation: fakeElevation(true)}
	if _, err := manager.Install(context.Background(), InstallRequest{INFPath: infPath, HardwareID: `PCI\VEN_1234&DEV_5678`}); err == nil {
		t.Fatal("unconfirmed installation must fail")
	}
}

func TestManagerRebootRequired(t *testing.T) {
	infPath := writeTestINF(t)
	installer := &fakeInstaller{outcome: InstallOutcome{ExitCode: 3010, RebootRequired: true}}
	manager := &Manager{Devices: &fakeDevices{}, Installer: installer, Verifier: fakeVerifier{signature: Signature{Verified: true}}, Elevation: fakeElevation(true)}
	result, err := manager.Install(context.Background(), InstallRequest{INFPath: infPath, HardwareID: `PCI\VEN_1234&DEV_5678`})
	if err != nil {
		t.Fatalf("reboot result must not be an error: %v", err)
	}
	if !result.RebootRequired || !result.Installed {
		t.Fatalf("unexpected reboot result: %+v", result)
	}
}

func TestManagerValidatesRequest(t *testing.T) {
	manager := &Manager{}
	if _, err := manager.Install(context.Background(), InstallRequest{}); err == nil {
		t.Fatal("empty request must fail")
	}
	text := filepath.Join(t.TempDir(), "not-a-driver.txt")
	if err := os.WriteFile(text, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Install(context.Background(), InstallRequest{INFPath: text, HardwareID: "PCI\\X"}); err == nil {
		t.Fatal("non-INF file must fail")
	}
}
