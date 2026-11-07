package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/windows"
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
	cache := t.TempDir()
	primary := filepath.Join(cache, "WinDeploy", "WebView2")
	if err := os.MkdirAll(primary, 0o700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(primary, "keep.txt")
	if err := os.WriteFile(marker, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, err := prepareDefaultWebViewDataDir(cache, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if dir != primary {
		t.Fatalf("primary profile was not used: %q", dir)
	}
	if data, err := os.ReadFile(marker); err != nil || string(data) != "keep" {
		t.Fatalf("existing profile data was disturbed: %q %v", data, err)
	}
}

func TestDefaultWebViewDataDirFallsBackWhenPrimaryIsBlocked(t *testing.T) {
	cache := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cache, "WinDeploy"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cache, "WinDeploy", "WebView2"), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	dir, err := prepareDefaultWebViewDataDir(cache, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(cache, "WinDeploy", "WebView2-fallback")
	if dir != want {
		t.Fatalf("unexpected fallback directory: %q (want %q)", dir, want)
	}
	if info, statErr := os.Stat(dir); statErr != nil || !info.IsDir() {
		t.Fatalf("fallback directory is unavailable: %v", statErr)
	}
}

func TestDefaultWebViewDataDirFallsBackToTemp(t *testing.T) {
	cache := t.TempDir()
	if err := os.MkdirAll(filepath.Join(cache, "WinDeploy"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"WebView2", "WebView2-fallback"} {
		if err := os.WriteFile(filepath.Join(cache, "WinDeploy", name), []byte("not a directory"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	temp := t.TempDir()
	dir, err := prepareDefaultWebViewDataDir(cache, temp)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(temp, "WinDeploy", "WebView2")
	if dir != want {
		t.Fatalf("unexpected temp fallback directory: %q (want %q)", dir, want)
	}
}

func TestPermissionFailureClassification(t *testing.T) {
	if !isPermissionFailure(fmt.Errorf("wrapped: %w", windows.ERROR_ACCESS_DENIED)) {
		t.Fatal("Windows access-denied error was not classified as a permission failure")
	}
	if !isPermissionFailure(fmt.Errorf("wrapped: %w", os.ErrPermission)) {
		t.Fatal("os.ErrPermission was not classified as a permission failure")
	}
	if isPermissionFailure(errors.New("disk full")) {
		t.Fatal("non-permission error was classified as a permission failure")
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
