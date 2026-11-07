//go:build windows

package atomicfile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/windows"
)

func writeTemp(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestReplaceOverwritesExistingTarget(t *testing.T) {
	dir := t.TempDir()
	target := writeTemp(t, dir, "session.json", "old")
	temp := writeTemp(t, dir, "session.tmp", "new")
	if err := Replace(context.Background(), temp, target); err != nil {
		t.Fatalf("replace failed: %v", err)
	}
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "new" {
		t.Fatalf("target not replaced: %q %v", data, err)
	}
}

// holdWithoutDelete opens the target without FILE_SHARE_DELETE so MoveFileEx
// cannot replace it. This reproduces the antivirus/indexer conflict that makes a
// session checkpoint fail with "Access is denied" on Windows.
func holdWithoutDelete(t *testing.T, path string) windows.Handle {
	t.Helper()
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	handle, err := windows.CreateFile(name, windows.GENERIC_READ, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatalf("could not hold the target: %v", err)
	}
	return handle
}

func TestReplaceRetriesTransientConflict(t *testing.T) {
	dir := t.TempDir()
	target := writeTemp(t, dir, "session.json", "old")
	temp := writeTemp(t, dir, "session.tmp", "new")
	handle := holdWithoutDelete(t, target)
	// Prove the setup really blocks a plain replacement on this file system.
	if err := os.Rename(temp, target); !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		windows.CloseHandle(handle)
		t.Skipf("file system does not report the sharing conflict as access denied: %v", err)
	}
	released := make(chan struct{})
	go func() {
		time.Sleep(45 * time.Millisecond)
		windows.CloseHandle(handle)
		close(released)
	}()
	if err := Replace(context.Background(), temp, target); err != nil {
		t.Fatalf("bounded retry did not recover: %v", err)
	}
	<-released
	data, err := os.ReadFile(target)
	if err != nil || string(data) != "new" {
		t.Fatalf("target not replaced: %q %v", data, err)
	}
}

func TestReplaceGivesUpOnPersistentConflict(t *testing.T) {
	dir := t.TempDir()
	target := writeTemp(t, dir, "session.json", "old")
	temp := writeTemp(t, dir, "session.tmp", "new")
	handle := holdWithoutDelete(t, target)
	defer windows.CloseHandle(handle)
	started := time.Now()
	err := Replace(context.Background(), temp, target)
	elapsed := time.Since(started)
	if err == nil {
		t.Fatal("persistent conflict reported success")
	}
	if !errors.Is(err, windows.ERROR_ACCESS_DENIED) {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed > replaceTotalLimit+time.Second {
		t.Fatalf("retry budget exceeded: %v", elapsed)
	}
	data, readErr := os.ReadFile(target)
	if readErr != nil || string(data) != "old" {
		t.Fatalf("failed replace damaged the target: %q %v", data, readErr)
	}
}

func TestReplaceHonorsCancellation(t *testing.T) {
	dir := t.TempDir()
	target := writeTemp(t, dir, "session.json", "old")
	temp := writeTemp(t, dir, "session.tmp", "new")
	handle := holdWithoutDelete(t, target)
	defer windows.CloseHandle(handle)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Replace(ctx, temp, target); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled replace returned %v", err)
	}
}
