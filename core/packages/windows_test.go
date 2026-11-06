//go:build windows

package packages

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func systemBinary(t *testing.T, name string) string {
	t.Helper()
	path := filepath.Join(os.Getenv("SystemRoot"), "System32", name)
	if _, err := os.Stat(path); err != nil {
		t.Skipf("system binary %s is unavailable", name)
	}
	return path
}

func signedApplication(t *testing.T) string {
	t.Helper()
	candidates := []string{
		filepath.Join(os.Getenv("ProgramFiles"), "nodejs", "node.exe"),
		`C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe`,
		filepath.Join(os.Getenv("ProgramFiles"), "Git", "cmd", "git.exe"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() {
			return candidate
		}
	}
	t.Skip("no embedded-signed vendor application is available for the signature test")
	return ""
}

func TestWindowsSignerVerifiesSystemBinary(t *testing.T) {
	path := signedApplication(t)
	signer := WindowsSigner{}
	signature, err := signer.Verify(context.Background(), path)
	if err != nil {
		t.Fatalf("signature verification error: %v", err)
	}
	if !signature.Verified {
		t.Fatalf("%s must have a trusted Authenticode signature", path)
	}
	if strings.TrimSpace(signature.Subject) == "" {
		t.Fatal("signer subject must be extracted from the embedded certificate")
	}
}

func TestWindowsSignerRejectsUnsignedFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unsigned.exe")
	if err := os.WriteFile(path, append([]byte("MZ"), make([]byte, 512)...), 0o644); err != nil {
		t.Fatal(err)
	}
	signature, err := (WindowsSigner{}).Verify(context.Background(), path)
	if err != nil {
		t.Fatalf("unexpected transport error: %v", err)
	}
	if signature.Verified {
		t.Fatal("a file without a signature must not be reported as verified")
	}
}

func TestWindowsFactsReadsFileMetadata(t *testing.T) {
	facts := WindowsFacts{}
	binary := systemBinary(t, "notepad.exe")
	info, err := facts.File(context.Background(), binary)
	if err != nil || !info.Exists {
		t.Fatalf("existing system binary not detected: %+v, %v", info, err)
	}
	if info.Version == "" {
		t.Fatal("system binary must expose a file version resource")
	}
	missing, err := facts.File(context.Background(), filepath.Join(os.Getenv("SystemRoot"), "System32", "windeploy-missing-test.exe"))
	if err != nil {
		t.Fatalf("missing file must not be an error: %v", err)
	}
	if missing.Exists {
		t.Fatal("missing file reported as present")
	}
	expanded := facts.ExpandPath(`%SystemRoot%\System32\notepad.exe`)
	if !strings.EqualFold(expanded, binary) {
		t.Fatalf("environment expansion failed: %q", expanded)
	}
}

func TestWindowsRunnerReportsExitCode(t *testing.T) {
	runner := WindowsRunner{}
	execution, err := runner.Run(context.Background(), Command{Path: "cmd.exe", Args: []string{"/c", "exit 7"}}, nil)
	if err != nil {
		t.Fatalf("runner error: %v", err)
	}
	if execution.ExitCode != 7 {
		t.Fatalf("exit code = %d, want 7", execution.ExitCode)
	}
	if execution.Output != "" {
		t.Fatalf("unexpected output: %q", execution.Output)
	}
}

func TestWindowsRunnerHonorsCancellation(t *testing.T) {
	runner := WindowsRunner{}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	_, err := runner.Run(ctx, Command{Path: "cmd.exe", Args: []string{"/c", "ping -n 10 127.0.0.1 > nul"}}, nil)
	if err == nil {
		t.Fatal("cancelled process must return an error")
	}
}

func TestWindowsElevationAndWingetProbe(t *testing.T) {
	elevated, err := (WindowsElevation{}).Elevated(context.Background())
	if err != nil {
		t.Fatalf("elevation probe failed: %v", err)
	}
	// The test process may run either elevated or not; the probe must be stable.
	if elevated && os.Getenv("WINDEPLOY_EXPECT_NONADMIN") == "1" {
		t.Fatal("elevation probe disagreed with the configured expectation")
	}
	probe := &WindowsWinget{}
	available, version := probe.Available(context.Background())
	if !available {
		candidates := wingetCandidates()
		if len(candidates) > 0 {
			t.Fatalf("winget candidates exist but the probe failed: %v", candidates)
		}
		return
	}
	if version == "" || len(version) > 64 {
		t.Fatalf("unexpected winget version: %q", version)
	}
	if probe.Path() == "" {
		t.Fatal("a successful winget probe must retain the executable path")
	}
	// A second probe must be served from the cache.
	again, againVersion := probe.Available(context.Background())
	if !again || againVersion != version {
		t.Fatal("cached winget probe disagreed with the first result")
	}
}
