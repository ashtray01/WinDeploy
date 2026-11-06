//go:build windows

package drivers

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"windeploy/core/inventory"
)

func TestWindowsDevicesEnumeratesPresentDevices(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	devices, err := (WindowsDevices{}).Devices(ctx)
	if err != nil {
		t.Fatalf("device enumeration failed: %v", err)
	}
	if len(devices) < 5 {
		t.Fatalf("suspiciously few devices: %d", len(devices))
	}
	withHardwareID, withDriver := 0, 0
	statuses := map[string]bool{}
	for _, device := range devices {
		if device.InstanceID == "" {
			t.Fatal("device without an instance ID")
		}
		if len(device.HardwareIDs) > 0 {
			withHardwareID++
		}
		if device.DriverVersion != "" {
			withDriver++
		}
		statuses[device.Status] = true
	}
	if withHardwareID == 0 {
		t.Fatal("no device reported a hardware ID")
	}
	if withDriver == 0 {
		t.Fatal("no device reported an installed driver version")
	}
	if !statuses["ok"] && !statuses["problem"] {
		t.Fatalf("unexpected device statuses: %+v", statuses)
	}
	t.Logf("devices=%d withHardwareID=%d withDriver=%d statuses=%v", len(devices), withHardwareID, withDriver, statuses)
}

func TestWindowsINFVerifierRejectsUnsignedINF(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unsigned.inf")
	if err := os.WriteFile(path, []byte(sampleINF), 0o644); err != nil {
		t.Fatal(err)
	}
	signature, err := (WindowsINFVerifier{}).VerifyINF(context.Background(), path)
	if err != nil {
		t.Fatalf("verification error: %v", err)
	}
	if signature.Verified {
		t.Fatal("unsigned INF must not be accepted")
	}
}

type recordingRunner struct {
	calls []Command
	exit  int
}

func (r *recordingRunner) Run(_ context.Context, command Command) (Execution, error) {
	r.calls = append(r.calls, command)
	return Execution{ExitCode: r.exit}, nil
}

func TestWindowsInstallerScanDevicesCommand(t *testing.T) {
	runner := &recordingRunner{}
	installer := WindowsInstaller{Runner: runner, System: inventory.SystemInfo{Windows: inventory.Version{Major: 10, Minor: 0, Build: 22631, ProductType: 1}}}
	if err := installer.ScanDevices(context.Background()); err != nil {
		t.Fatalf("scan failed: %v", err)
	}
	if len(runner.calls) != 1 || len(runner.calls[0].Args) != 1 || runner.calls[0].Args[0] != "/scan-devices" {
		t.Fatalf("unexpected pnputil call: %+v", runner.calls)
	}
	win7 := WindowsInstaller{Runner: runner, System: inventory.SystemInfo{Windows: inventory.Version{Major: 6, Minor: 1, Build: 7601, ProductType: 1}}}
	if err := win7.ScanDevices(context.Background()); err == nil {
		t.Fatal("Windows 7 rescan must be reported as unsupported")
	}
}

func TestWindowsInstallerFindsPnputil(t *testing.T) {
	if path := findPnputil(); path == "" {
		t.Fatal("pnputil.exe is required for driver installation")
	} else if !strings.HasSuffix(strings.ToLower(path), "pnputil.exe") {
		t.Fatalf("unexpected pnputil path: %q", path)
	}
	if _, err := (WindowsElevation{}).Elevated(context.Background()); err != nil {
		t.Fatalf("elevation probe failed: %v", err)
	}
}
