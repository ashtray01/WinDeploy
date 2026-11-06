package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"windeploy/core/engine"
	"windeploy/core/inventory"
)

type fixtureProvider struct{ partial bool }

func (f fixtureProvider) System(context.Context) (inventory.SystemInfo, error) {
	return inventory.SystemInfo{Hostname: "test", Architecture: "amd64"}, nil
}
func (f fixtureProvider) CPU(context.Context) (inventory.CPUInfo, error) {
	if f.partial {
		return inventory.CPUInfo{}, errors.New("secret-input")
	}
	return inventory.CPUInfo{Model: "Test", LogicalProcessors: 2}, nil
}
func (f fixtureProvider) Memory(context.Context) (inventory.MemoryInfo, error) {
	return inventory.MemoryInfo{TotalPhysicalBytes: 8 << 30}, nil
}
func (f fixtureProvider) Volumes(context.Context) ([]inventory.Volume, error) { return nil, nil }
func (f fixtureProvider) Interfaces(context.Context) ([]inventory.InterfaceInfo, error) {
	return nil, nil
}
func (f fixtureProvider) NetworkConfig(context.Context) ([]inventory.AdapterConfig, error) {
	return nil, nil
}
func (f fixtureProvider) Apps(context.Context) ([]inventory.InstalledApp, error) { return nil, nil }

func TestInventoryCLI(t *testing.T) {
	for _, partial := range []bool{false, true} {
		var out, stderr bytes.Buffer
		code := run(context.Background(), []string{"inventory", "--format", "json"}, &out, &stderr, fixtureProvider{partial: partial})
		want := 0
		if partial {
			want = 2
		}
		if code != want {
			t.Fatalf("code %d, stderr=%s", code, stderr.String())
		}
		var report inventory.Report
		if err := json.Unmarshal(out.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report.Complete == partial || strings.Contains(out.String()+stderr.String(), "secret-input") {
			t.Fatal("invalid or unsafe report")
		}
	}
	var out, stderr bytes.Buffer
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if code := run(ctx, []string{"inventory", "--format", "json"}, &out, &stderr, fixtureProvider{}); code != 3 {
		t.Fatalf("cancel code=%d", code)
	}
	for _, args := range [][]string{{"execute"}, {"inventory", "--format", "bad"}, {"inventory", "--timeout", "0s"}, {"preview"}} {
		if code := run(context.Background(), args, &out, &stderr, fixtureProvider{}); code != 1 {
			t.Fatalf("invalid command accepted: %v", args)
		}
	}
}

func TestPreviewCLIAndExclusiveOutput(t *testing.T) {
	dir := t.TempDir()
	yaml := "version: 1\nid: test\nname: Test\ncomponents:\n  - id: check\n    name: Check\n    category: diagnostics\n    parameters: {kind: private-config}\n"
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	var out, stderr bytes.Buffer
	if code := run(context.Background(), []string{"preview", "--profiles", dir, "--profile", "test"}, &out, &stderr, nil); code != 0 {
		t.Fatalf("code=%d %s", code, stderr.String())
	}
	var preview engine.PlanPreview
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Tasks) != 1 || strings.Contains(out.String(), "private-config") || strings.Contains(out.String(), "parameters") {
		t.Fatal("unsafe preview")
	}
	out.Reset()
	if code := run(context.Background(), []string{"preview", "--profiles", dir, "--profile", "test", "--select="}, &out, &stderr, nil); code != 0 {
		t.Fatal(stderr.String())
	}
	if err := json.Unmarshal(out.Bytes(), &preview); err != nil || len(preview.Tasks) != 0 {
		t.Fatal("explicit empty selection ignored")
	}
	path := filepath.Join(dir, "result.json")
	if err := writeOutput(path, []byte("original"), &out); err != nil {
		t.Fatal(err)
	}
	if err := writeOutput(path, []byte("replacement"), &out); err == nil {
		t.Fatal("existing report overwritten")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "original" {
		t.Fatal("report changed")
	}
	if code := run(context.Background(), []string{"inventory", "--output", path}, &out, &stderr, fixtureProvider{}); code != 1 {
		t.Fatal("output failure ignored")
	}
	if err := os.WriteFile(filepath.Join(dir, "duplicate.yaml"), []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadProfiles(context.Background(), dir); err == nil {
		t.Fatal("duplicate profiles accepted")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func TestCLIWriterErrors(t *testing.T) {
	var stderr bytes.Buffer
	if code := run(context.Background(), []string{"inventory"}, brokenWriter{}, &stderr, fixtureProvider{}); code != 1 {
		t.Fatal("stdout error ignored")
	}
}
