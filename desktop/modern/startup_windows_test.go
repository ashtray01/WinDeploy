package main

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
