package main

import (
	"bytes"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"windeploy/internal/trusttest"
)

func TestStartupLogPreservesDiagnosticsWhenFileCannotOpen(t *testing.T) {
	previous, path := log.Writer(), startupLogPath
	t.Cleanup(func() { log.SetOutput(previous); startupLogPath = path })
	dir := t.TempDir()
	// A directory where a regular log file is expected deterministically fails.
	if err := os.Mkdir(filepath.Join(dir, "startup.log"), 0o700); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	file, err := configureStartupLog(dir, &stderr)
	if err == nil || file != nil {
		t.Fatalf("expected log failure, file=%v err=%v", file, err)
	}
	logStation("startup failure evidence")
	log.Print("framework failure evidence")
	if !strings.Contains(stderr.String(), "startup failure evidence") || !strings.Contains(stderr.String(), "framework failure evidence") {
		t.Fatalf("diagnostics lost: %q", stderr.String())
	}
}

func TestStartupLogWritesConsoleAndFile(t *testing.T) {
	previous, path := log.Writer(), startupLogPath
	t.Cleanup(func() { log.SetOutput(previous); startupLogPath = path })
	var stderr bytes.Buffer
	file, err := configureStartupLog(t.TempDir(), &stderr)
	if err != nil {
		t.Fatal(err)
	}
	logStation("startup evidence")
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(startupLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "startup evidence") || !strings.Contains(stderr.String(), "startup evidence") {
		t.Fatalf("file=%q stderr=%q", data, stderr.String())
	}
}

func TestStartupLogWritesFileWhenConsoleIsUnavailable(t *testing.T) {
	previous, path := log.Writer(), startupLogPath
	t.Cleanup(func() { log.SetOutput(previous); startupLogPath = path })
	file, err := configureStartupLog(t.TempDir(), failingWriter{})
	if err != nil {
		t.Fatal(err)
	}
	logStation("file-only evidence")
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(startupLogPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "file-only evidence") {
		t.Fatalf("GUI startup evidence was lost: %q", data)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, os.ErrClosed }

func TestDefaultWebViewDataDirUsesWritablePrimary(t *testing.T) {
	cache := trusttest.Dir(t)
	primary := filepath.Join(cache, "WinDeploy", "WebView2")
	if err := os.MkdirAll(primary, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(primary, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, fallback, err := prepareDefaultWebViewDataDir(cache, trusttest.Dir(t), trusttest.Dir(t))
	if err != nil {
		t.Fatal(err)
	}
	if dir != primary {
		t.Fatalf("primary profile was not used: %q", dir)
	}
	if fallback {
		t.Fatal("primary profile was incorrectly marked as fallback")
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "keep" {
		t.Fatalf("existing profile data was disturbed: %q %v", data, err)
	}
}

func TestDefaultWebViewDataDirFallsBackWhenPrimaryIsBlocked(t *testing.T) {
	cache := trusttest.Dir(t)
	if err := os.MkdirAll(filepath.Join(cache, "WinDeploy"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "WinDeploy", "WebView2"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, fallback, err := prepareDefaultWebViewDataDir(cache, trusttest.Dir(t), trusttest.Dir(t))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cache, "WinDeploy", "WebView2-fallback")
	if dir != want {
		t.Fatalf("unexpected fallback directory: %q (want %q)", dir, want)
	}
	if !fallback {
		t.Fatal("fallback directory was not marked as fallback")
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		t.Fatalf("fallback directory is unavailable: %v", statErr)
	}
}

func TestDefaultWebViewDataDirFallsBackToTemp(t *testing.T) {
	cache := trusttest.Dir(t)
	if err := os.MkdirAll(filepath.Join(cache, "WinDeploy"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"WebView2", "WebView2-fallback"} {
		if err := os.WriteFile(filepath.Join(cache, "WinDeploy", name), []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	temp := trusttest.Dir(t)
	dir, fallback, err := prepareDefaultWebViewDataDir(cache, temp, trusttest.Dir(t))
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(temp, "WinDeploy", "WebView2")
	if dir != want {
		t.Fatalf("unexpected temp fallback directory: %q (want %q)", dir, want)
	}
	if !fallback {
		t.Fatal("temp fallback directory was not marked as fallback")
	}
}

func TestFallbackMarkerIsReusable(t *testing.T) {
	dir := trusttest.Dir(t)
	if fallbackAccepted(dir) {
		t.Fatal("fresh fallback directory was already accepted")
	}
	if err := markFallbackAccepted(dir); err != nil {
		t.Fatal(err)
	}
	if !fallbackAccepted(dir) {
		t.Fatal("fallback acceptance marker was not persisted")
	}
	data, err := os.ReadFile(filepath.Join(dir, fallbackMarkerName))
	if err != nil || strings.Contains(string(data), "password") {
		t.Fatalf("unexpected marker: %q %v", data, err)
	}
}

func TestTrustedFallbackRejectsWritableByUsers(t *testing.T) {
	if _, err := exec.LookPath("icacls.exe"); err != nil {
		t.Skip("icacls is unavailable")
	}
	dir := trusttest.Dir(t)
	if output, err := exec.Command("icacls.exe", dir, "/grant", "*S-1-5-32-545:(OI)(CI)(M)").CombinedOutput(); err != nil {
		t.Skipf("could not add a test ACE: %v %s", err, output)
	}
	if _, err := prepareTrustedWebViewDataDir(dir); err == nil {
		t.Fatal("fallback writable by Users was accepted")
	}
}

func TestTrustedFallbackAllowsReadOnlyUsers(t *testing.T) {
	if _, err := exec.LookPath("icacls.exe"); err != nil {
		t.Skip("icacls is unavailable")
	}
	dir := trusttest.Dir(t)
	if output, err := exec.Command("icacls.exe", dir, "/grant", "*S-1-5-32-545:(OI)(CI)(RX)").CombinedOutput(); err != nil {
		t.Skipf("could not add a read-only test ACE: %v %s", err, output)
	}
	if _, err := prepareTrustedWebViewDataDir(dir); err != nil {
		t.Fatalf("read-only fallback was rejected: %v", err)
	}
}

func TestProgramFilesPortableFallbackIsSkipped(t *testing.T) {
	programFiles := strings.TrimSpace(os.Getenv("ProgramFiles"))
	if programFiles == "" {
		t.Skip("ProgramFiles is unavailable")
	}
	if !isProgramFilesPath(filepath.Join(programFiles, "WinDeploy")) {
		t.Fatal("Program Files path was not recognized")
	}
	cache := trusttest.Dir(t)
	if err := os.MkdirAll(filepath.Join(cache, "WinDeploy"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"WebView2", "WebView2-fallback"} {
		if err := os.WriteFile(filepath.Join(cache, "WinDeploy", name), []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	blockedTemp := filepath.Join(trusttest.Dir(t), "blocked")
	if err := os.WriteFile(blockedTemp, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := prepareDefaultWebViewDataDir(cache, blockedTemp, programFiles); err == nil {
		t.Fatal("portable fallback under Program Files was attempted")
	}
}

func TestDefaultWebViewDataDirReusesFallback(t *testing.T) {
	cache := trusttest.Dir(t)
	if err := os.MkdirAll(filepath.Join(cache, "WinDeploy"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "WinDeploy", "WebView2"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	temp := trusttest.Dir(t)
	exeDir := trusttest.Dir(t)
	first, fallback, err := prepareDefaultWebViewDataDir(cache, temp, exeDir)
	if err != nil || !fallback {
		t.Fatalf("first selection: dir=%q fallback=%v err=%v", first, fallback, err)
	}
	if err := os.WriteFile(filepath.Join(first, "keep.txt"), []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	second, fallback, err := prepareDefaultWebViewDataDir(cache, temp, exeDir)
	if err != nil || !fallback || first != second {
		t.Fatalf("fallback was not reused: first=%q second=%q fallback=%v err=%v", first, second, fallback, err)
	}
	if data, err := os.ReadFile(filepath.Join(second, "keep.txt")); err != nil || string(data) != "keep" {
		t.Fatalf("fallback state was not preserved: %q %v", data, err)
	}
}

func TestDefaultWebViewDataDirFallsBackNextToExecutable(t *testing.T) {
	cache := trusttest.Dir(t)
	if err := os.MkdirAll(filepath.Join(cache, "WinDeploy"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"WebView2", "WebView2-fallback"} {
		if err := os.WriteFile(filepath.Join(cache, "WinDeploy", name), []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	blockedTemp := filepath.Join(trusttest.Dir(t), "blocked")
	if err := os.WriteFile(blockedTemp, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	executableDir := trusttest.Dir(t)
	dir, fallback, err := prepareDefaultWebViewDataDir(cache, blockedTemp, executableDir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(executableDir, "WinDeploy-WebView2Data")
	if dir != want {
		t.Fatalf("unexpected portable fallback directory: %q (want %q)", dir, want)
	}
	if !fallback {
		t.Fatal("portable fallback directory was not marked as fallback")
	}
}

func TestWebViewDataDirectoryProbePreservesExistingData(t *testing.T) {
	dir := t.TempDir()
	existing := filepath.Join(dir, "existing.txt")
	if err := os.WriteFile(existing, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	actual, err := prepareWebViewDataDir(dir)
	if err != nil || actual != dir {
		t.Fatalf("dir=%q err=%v", actual, err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 1 || files[0].Name() != "existing.txt" {
		t.Fatalf("unexpected contents: %v %v", files, err)
	}
	if _, err := prepareWebViewDataDir(existing); err == nil {
		t.Fatal("regular file accepted as data directory")
	}
}

func TestWebViewDataDirectoryRejectsJunctionAncestor(t *testing.T) {
	if _, err := exec.LookPath("cmd.exe"); err != nil {
		t.Skip("cmd.exe is unavailable")
	}
	root := t.TempDir()
	target := filepath.Join(root, "target")
	if err := os.Mkdir(target, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
		t.Skipf("junctions are unavailable: %v %s", err, output)
	}
	if _, err := prepareWebViewDataDir(filepath.Join(link, "profile")); err == nil {
		t.Fatal("junction ancestor was accepted")
	}
}
