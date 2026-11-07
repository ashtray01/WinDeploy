//go:build windows

package trust

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
	"windeploy/internal/trusttest"
)

// These tests use real Windows ACLs on files inside private test directories. They
// never change ACLs outside the test directory and never take ownership of foreign
// objects.

func TestCheckAcceptsOwnedPrivateState(t *testing.T) {
	dir := trusttest.Dir(t)
	if err := Check(context.Background(), dir, true); err != nil {
		t.Fatalf("private test directory rejected: %v", err)
	}
	file := filepath.Join(dir, "session.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Check(context.Background(), file, false); err != nil {
		t.Fatalf("owned session file rejected: %v", err)
	}
}

func TestCheckRejectsMissingAndWrongType(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "state.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Check(context.Background(), filepath.Join(dir, "missing.json"), false); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("missing path must be untrusted: %v", err)
	}
	if err := Check(context.Background(), file, true); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("file as directory must be untrusted: %v", err)
	}
	if err := Check(context.Background(), dir, false); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("directory as file must be untrusted: %v", err)
	}
}

func TestCheckRejectsWritableByUsers(t *testing.T) {
	if _, err := exec.LookPath("icacls.exe"); err != nil {
		t.Skip("icacls is unavailable")
	}
	dir := trusttest.Dir(t)
	file := filepath.Join(dir, "state.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Grant the built-in Users group write access to this test file only.
	if output, err := exec.Command("icacls.exe", file, "/grant", "*S-1-5-32-545:(W)").CombinedOutput(); err != nil {
		t.Skipf("could not add a test ACE: %v %s", err, output)
	}
	if err := Check(context.Background(), file, false); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("file writable by Users must be untrusted: %v", err)
	}
}

func TestCheckAllowsReadOnlyAccessForUsers(t *testing.T) {
	if _, err := exec.LookPath("icacls.exe"); err != nil {
		t.Skip("icacls is unavailable")
	}
	dir := trusttest.Dir(t)
	file := filepath.Join(dir, "state.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	// A read/execute ACE contains READ_CONTROL and SYNCHRONIZE. Those common
	// standard rights must not be mistaken for FILE_GENERIC_WRITE.
	if output, err := exec.Command("icacls.exe", file, "/grant", "*S-1-5-32-545:(RX)").CombinedOutput(); err != nil {
		t.Skipf("could not add a read-only test ACE: %v %s", err, output)
	}
	if err := Check(context.Background(), file, false); err != nil {
		t.Fatalf("file with read-only Users access was rejected: %v", err)
	}
}

func TestCheckAllowsAdministratorsAsElevatedDefaultOwner(t *testing.T) {
	dir := trusttest.Dir(t)
	file := filepath.Join(dir, "elevated-state.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	administrators, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(file, windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION, administrators, nil, nil, nil); err != nil {
		t.Skipf("cannot assign elevated default owner: %v", err)
	}
	if err := Check(context.Background(), file, false); err != nil {
		t.Fatalf("Administrators-owned file with private DACL was rejected: %v", err)
	}
}

func TestCheckRejectsDirectoryJunction(t *testing.T) {
	if _, err := exec.LookPath("cmd.exe"); err != nil {
		t.Skip("cmd.exe is unavailable")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "junction")
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("junctions are unavailable: %v %s", err, output)
	}
	if err := Check(context.Background(), link, true); !errors.Is(err, ErrUntrusted) {
		t.Fatalf("junction must be untrusted: %v", err)
	}
}
