package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"windeploy/core/inventory"
	"windeploy/core/packages"
)

type cliTestFacts struct {
	apps []inventory.InstalledApp
}

func (f *cliTestFacts) Apps(context.Context) ([]inventory.InstalledApp, error) {
	return append([]inventory.InstalledApp{}, f.apps...), nil
}

func (f *cliTestFacts) File(context.Context, string) (packages.FileInfo, error) {
	return packages.FileInfo{}, nil
}

func (f *cliTestFacts) ExpandPath(path string) string { return path }

type cliTestFetcher struct {
	data  []byte
	count int
}

func (f *cliTestFetcher) Download(_ context.Context, request packages.DownloadRequest) (packages.DownloadResult, error) {
	f.count++
	if err := os.MkdirAll(filepath.Dir(request.Destination), 0o755); err != nil {
		return packages.DownloadResult{}, err
	}
	if err := os.WriteFile(request.Destination, f.data, 0o644); err != nil {
		return packages.DownloadResult{}, err
	}
	digest := packages.HashBytes(f.data)
	if request.ExpectedSHA256 != "" && !strings.EqualFold(digest, request.ExpectedSHA256) {
		return packages.DownloadResult{}, packages.ErrHashMismatch
	}
	return packages.DownloadResult{Path: request.Destination, Size: int64(len(f.data)), SHA256: digest}, nil
}

type cliTestRunner struct{}

func (cliTestRunner) Run(context.Context, packages.Command, func(string)) (packages.Execution, error) {
	return packages.Execution{ExitCode: 0}, nil
}

type cliTestSigner struct{}

func (cliTestSigner) Verify(context.Context, string) (packages.Signature, error) {
	return packages.Signature{Verified: true, Signer: "Test"}, nil
}

type cliTestElevation bool

func (e cliTestElevation) Elevated(context.Context) (bool, error) { return bool(e), nil }

type cliTestWinget struct{}

func (cliTestWinget) Available(context.Context) (bool, string) { return true, "1.9.0" }

func cliTestAdapters(cacheDir string) cliAdapters {
	return cliAdapters{
		facts: &cliTestFacts{}, fetcher: &cliTestFetcher{data: []byte("artifact")}, runner: cliTestRunner{},
		signer: cliTestSigner{}, elevation: cliTestElevation(true), winget: cliTestWinget{},
		cacheDir: cacheDir,
		system:   inventory.SystemInfo{Architecture: "amd64", Windows: inventory.Version{Major: 10, Minor: 0, Build: 22631, ProductType: 1}},
	}
}

func runCLI(t *testing.T, cacheDir string, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr, cliTestAdapters(cacheDir))
	return code, stdout.String(), stderr.String()
}

func TestListShowsEmbeddedCatalog(t *testing.T) {
	code, stdout, _ := runCLI(t, t.TempDir(), "list")
	if code != exitOK {
		t.Fatalf("list exit = %d", code)
	}
	if !strings.Contains(stdout, "sevenzip") || !strings.Contains(stdout, "7-Zip") {
		t.Fatalf("catalog listing is incomplete:\n%s", stdout)
	}
	if !strings.Contains(stdout, "printerone") {
		t.Fatalf("printerone missing from listing:\n%s", stdout)
	}
}

func TestPlanIsReadOnlyAndInvalidInputFails(t *testing.T) {
	code, stdout, _ := runCLI(t, t.TempDir(), "plan", "-id", "sevenzip")
	if code != exitOK || !strings.Contains(stdout, "sevenzip") {
		t.Fatalf("plan failed: exit=%d output=%s", code, stdout)
	}
	code, _, _ = runCLI(t, t.TempDir(), "plan", "-id", "missing-package")
	if code != exitPlan {
		t.Fatalf("unknown package exit = %d, want %d", code, exitPlan)
	}
	code, _, _ = runCLI(t, t.TempDir(), "unknown-command")
	if code != exitUsage {
		t.Fatalf("unknown command exit = %d, want %d", code, exitUsage)
	}
}

func TestInstallRequiresConfirmation(t *testing.T) {
	code, _, stderr := runCLI(t, t.TempDir(), "install", "-id", "sevenzip")
	if code != exitConfirm || !strings.Contains(stderr, "--yes") {
		t.Fatalf("install without confirmation: exit=%d stderr=%s", code, stderr)
	}
}

func TestInstallDryRunDoesNotExecute(t *testing.T) {
	code, stdout, _ := runCLI(t, t.TempDir(), "install", "-id", "sevenzip", "-dry-run")
	if code != exitOK {
		t.Fatalf("dry-run exit = %d output=%s", code, stdout)
	}
	if !strings.Contains(stdout, "skipped") {
		t.Fatalf("dry-run should report skipped operations:\n%s", stdout)
	}
}

func TestDownloadUsesCatalogHashAndCache(t *testing.T) {
	cacheDir := t.TempDir()
	catalogPath := filepath.Join(t.TempDir(), "catalog.yaml")
	data := []byte("artifact")
	catalog := `version: 1
packages:
  - id: test-package
    name: Test Package
    publisher: Test
    category: utilities
    requirements:
      versions: [windows-11]
      architectures: [amd64]
    detection:
      - kind: uninstall
        name_pattern: '^Test Package$'
    sources:
      - id: http-source
        kind: http
        url: https://example.test/setup.exe
        type: exe
        version: '1.0'
        sha256: ` + packages.HashBytes(data) + `
        priority: 0
`
	if err := os.WriteFile(catalogPath, []byte(catalog), 0o644); err != nil {
		t.Fatal(err)
	}
	adapters := cliTestAdapters(cacheDir)
	adapters.fetcher = &cliTestFetcher{data: data}
	adapters.winget = nil
	var stdout, stderr bytes.Buffer
	code := run([]string{"download", "-catalog", catalogPath, "-id", "test-package", "-cache", cacheDir}, &stdout, &stderr, adapters)
	if code != exitOK {
		t.Fatalf("download exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "success") || !strings.Contains(stdout.String(), "нет") {
		t.Fatalf("unexpected download output:\n%s", stdout.String())
	}
	entries, err := os.ReadDir(filepath.Join(cacheDir, "sha256"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("cache content missing: %v, entries=%d", err, len(entries))
	}
}
