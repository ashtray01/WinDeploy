//go:build windows

package shares

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWindowsCredentialStoreRoundTrip(t *testing.T) {
	store := WindowsCredentialStore{}
	ctx := context.Background()
	target := fmt.Sprintf("WinDeploy:share:windeploy-test-%d", time.Now().UnixNano())
	if err := store.Save(ctx, target, `WINDEPLOY\testuser`, "s3cret-pass"); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	defer store.Delete(ctx, target)
	credential, ok, err := store.Read(ctx, target)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if !ok || credential.Username != `WINDEPLOY\testuser` || credential.Password != "s3cret-pass" {
		t.Fatalf("unexpected credential: %+v ok=%t", credential, ok)
	}
	if err := store.Delete(ctx, target); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if _, ok, err := store.Read(ctx, target); err != nil || ok {
		t.Fatalf("credential still present: ok=%t err=%v", ok, err)
	}
}

func TestWindowsConnectorListsWithoutError(t *testing.T) {
	connections, err := (WindowsConnector{}).List(context.Background())
	if err != nil {
		t.Fatalf("enumeration failed: %v", err)
	}
	for _, connection := range connections {
		if connection.RemoteName == "" {
			t.Fatalf("connection without remote name: %+v", connection)
		}
	}
}

func TestWindowsShortcutCreatesLnk(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Тестовая папка.lnk")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if err := (WindowsShortcut{}).Create(ctx, ShortcutSpec{Path: path, Target: `\\server\share`, Description: "Тест"}); err != nil {
		t.Fatalf("shortcut creation failed: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Size() < 0x4C {
		t.Fatalf("shortcut file is missing or too small: %v, size=%d", err, infoSize(info))
	}
	header := make([]byte, 4)
	file, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := file.Read(header); err != nil {
		t.Fatal(err)
	}
	if header[0] != 0x4C || header[1] != 0x00 || header[2] != 0x00 || header[3] != 0x00 {
		t.Fatalf("not a .lnk header: %x", header)
	}
}

func infoSize(info os.FileInfo) int64 {
	if info == nil {
		return 0
	}
	return info.Size()
}

func TestDesktopDirectoryIsReal(t *testing.T) {
	dir, err := DesktopDirectory()
	if err != nil {
		t.Fatalf("desktop directory failed: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		t.Fatalf("desktop directory is not accessible: %v", err)
	}
}
