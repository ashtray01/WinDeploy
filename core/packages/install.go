package packages

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"windeploy/core/inventory"
)

// Status is the final state of one package operation.
type Status string

const (
	StatusSuccess        Status = "success"
	StatusUpToDate       Status = "up-to-date"
	StatusSkipped        Status = "skipped"
	StatusFailed         Status = "failed"
	StatusCancelled      Status = "cancelled"
	StatusRebootRequired Status = "reboot-required"
	StatusUnavailable    Status = "unavailable"
	StatusIncompatible   Status = "incompatible"
	StatusUnknown        Status = "unknown"
)

// Stable error codes are safe for UI translation.
const (
	CodeElevationRequired = "elevation-required"
	CodeAdminUnavailable  = "elevation-unknown"
	CodeDownloadFailed    = "download-failed"
	CodeHashMismatch      = "hash-mismatch"
	CodeSignatureFailed   = "signature-failed"
	CodeSourceUnavailable = "source-unavailable"
	CodeInstallFailed     = "install-failed"
	CodeVerifyFailed      = "verify-failed"
	CodeCancelled         = "cancelled"
	CodeTimeout           = "timeout"
	CodeUnsupported       = "unsupported-platform"
	CodeUnknownSource     = "unknown-source"
	CodeDryRun            = "dry-run"
)

// ErrElevationRequired is returned before any system change when a selected
// package needs an elevated token and the current process is not elevated.
var ErrElevationRequired = errors.New("administrator privileges are required")

// ErrSignatureInvalid reports an unsigned, untrusted or mismatched publisher
// signature for an artifact that requires Authenticode verification.
var ErrSignatureInvalid = errors.New("artifact signature is not trusted")

// Command is one local process invocation. Path without a separator is resolved
// through PATH; Args are passed verbatim without a shell.
type Command struct {
	Path string
	Args []string
	Dir  string
}

// Execution is a bounded process outcome. Output is a sanitized tail.
type Execution struct {
	ExitCode int
	Output   string
}

// Runner executes local processes and honors context cancellation. On cancellation
// Windows terminates the direct child; installers may leave their own child
// processes running, which is documented as a limitation.
type Runner interface {
	Run(context.Context, Command, func(string)) (Execution, error)
}

// Signature is the Authenticode outcome for an artifact.
type Signature struct {
	Verified bool
	Signer   string
	Subject  string
}

// Signer verifies an Authenticode signature. An untrusted or unsigned file must
// return Verified=false.
type Signer interface {
	Verify(context.Context, string) (Signature, error)
}

// Elevation reports whether the current process token is elevated.
type Elevation interface {
	Elevated(context.Context) (bool, error)
}

// Event is one structured, sanitized progress record.
type Event struct {
	Time      time.Time `json:"time"`
	Level     string    `json:"level"`
	Code      string    `json:"code"`
	PackageID string    `json:"package_id,omitempty"`
	Detail    string    `json:"detail,omitempty"`
}

// InstallOptions controls one run. DryRun resolves sources without changing the
// system and without downloading.
type InstallOptions struct {
	DryRun          bool
	DownloadTimeout time.Duration
	InstallTimeout  time.Duration
	MaxAttempts     int
	RetryDelay      time.Duration
}

// Result describes one package operation. Error is a sanitized diagnostic.
type Result struct {
	PackageID       string    `json:"package_id"`
	Name            string    `json:"name"`
	Action          Action    `json:"action"`
	Status          Status    `json:"status"`
	SourceID        string    `json:"source_id,omitempty"`
	SourceVersion   string    `json:"source_version,omitempty"`
	InstalledBefore string    `json:"installed_before,omitempty"`
	InstalledAfter  string    `json:"installed_after,omitempty"`
	ExitCode        int       `json:"exit_code,omitempty"`
	Attempts        int       `json:"attempts"`
	RebootRequired  bool      `json:"reboot_required"`
	ErrorCode       string    `json:"error_code,omitempty"`
	Error           string    `json:"error,omitempty"`
	ArtifactSHA256  string    `json:"artifact_sha256,omitempty"`
	StartedAt       time.Time `json:"started_at"`
	FinishedAt      time.Time `json:"finished_at"`
}

// Run is the complete outcome of Install.
type Run struct {
	StartedAt      time.Time `json:"started_at"`
	FinishedAt     time.Time `json:"finished_at"`
	Results        []Result  `json:"results"`
	RebootRequired bool      `json:"reboot_required"`
	Cancelled      bool      `json:"cancelled"`
	DryRun         bool      `json:"dry_run"`
}

// Manager coordinates detection, download, verification and installation. All
// platform effects are delegated to injected adapters, so the logic is unit tested
// without touching a real machine.
type Manager struct {
	Catalog   Catalog
	Facts     Facts
	System    inventory.SystemInfo
	Winget    WingetProbe
	Fetcher   Fetcher
	Runner    Runner
	Signer    Signer
	Elevation Elevation
	CacheDir  string
	Report    func(Event)
}

// Install plans and executes the selected packages sequentially. Packages that
// are already current are skipped. A failure never stops independent packages.
func (m *Manager) Install(ctx context.Context, selection []string, options InstallOptions) (Run, error) {
	if m.Facts == nil {
		return Run{}, fmt.Errorf("facts provider is required")
	}
	if err := m.Catalog.Validate(); err != nil {
		return Run{}, err
	}
	planner := Planner{Catalog: m.Catalog, Facts: m.Facts, System: m.System, Winget: m.Winget}
	// Planning is read-only and bounded separately, so a cancellation arriving
	// before the first operation still produces explicit Cancelled results
	// instead of an error without a run.
	planCtx, stopPlanning := context.WithTimeout(context.Background(), 2*time.Minute)
	plan, err := planner.Build(planCtx, selection)
	stopPlanning()
	if err != nil {
		return Run{}, err
	}
	if !options.DryRun {
		needsAdmin := false
		for _, item := range plan.Items {
			if item.Installable() && item.RequiresAdmin {
				needsAdmin = true
				break
			}
		}
		if needsAdmin {
			if m.Elevation == nil {
				return Run{}, ErrElevationRequired
			}
			elevated, elevateErr := m.Elevation.Elevated(ctx)
			if elevateErr != nil {
				return Run{}, fmt.Errorf("administrator state is unknown")
			}
			if !elevated {
				m.emit("error", "package.elevation-required", "", "")
				return Run{}, ErrElevationRequired
			}
		}
	}
	run := Run{StartedAt: time.Now().UTC(), Results: []Result{}, DryRun: options.DryRun}
	for _, item := range plan.Items {
		if err := ctx.Err(); err != nil {
			now := time.Now().UTC()
			run.Results = append(run.Results, Result{PackageID: item.PackageID, Name: item.Name, Action: item.Action, Status: StatusCancelled, ErrorCode: CodeCancelled, StartedAt: now, FinishedAt: now})
			run.Cancelled = true
			continue
		}
		result := m.runItem(ctx, item, options)
		if result.Status == StatusCancelled {
			run.Cancelled = true
		}
		run.Results = append(run.Results, result)
		run.RebootRequired = run.RebootRequired || result.RebootRequired
	}
	run.FinishedAt = time.Now().UTC()
	return run, nil
}

func (m *Manager) emit(level, code, packageID, detail string) {
	if m.Report == nil {
		return
	}
	m.Report(Event{Time: time.Now().UTC(), Level: level, Code: code, PackageID: packageID, Detail: detail})
}

func (m *Manager) runItem(ctx context.Context, item PlanItem, options InstallOptions) Result {
	result := Result{PackageID: item.PackageID, Name: item.Name, Action: item.Action, SourceID: item.SourceID, SourceVersion: item.SourceVersion, InstalledBefore: item.Installed.Version, StartedAt: time.Now().UTC()}
	result.FinishedAt = result.StartedAt
	m.emit("info", "package.plan", item.PackageID, string(item.Action))
	switch item.Action {
	case ActionUpToDate:
		result.Status = StatusUpToDate
		m.emit("info", "package.already-installed", item.PackageID, detailVersion(item.Installed.Version))
		return result
	case ActionUnavailable:
		result.Status, result.ErrorCode = StatusUnavailable, CodeSourceUnavailable
		m.emit("warning", "package.source-unavailable", item.PackageID, item.ReasonCode)
		return result
	case ActionIncompatible:
		result.Status, result.ErrorCode = StatusIncompatible, "os-incompatible"
		m.emit("warning", "package.incompatible", item.PackageID, "")
		return result
	case ActionUnknown:
		result.Status, result.ErrorCode = StatusUnknown, item.ReasonCode
		m.emit("warning", "package.unknown", item.PackageID, item.ReasonCode)
		return result
	case ActionInstall, ActionUpdate:
	default:
		result.Status, result.ErrorCode = StatusUnknown, "invalid-action"
		return result
	}
	pkg, ok := m.Catalog.Find(item.PackageID)
	if !ok {
		result.Status, result.ErrorCode = StatusUnknown, CodeUnknownSource
		return result
	}
	source, ok := findSource(pkg, item.SourceID)
	if !ok {
		result.Status, result.ErrorCode = StatusUnknown, CodeUnknownSource
		return result
	}
	if options.DryRun {
		result.Status, result.ErrorCode = StatusSkipped, CodeDryRun
		m.emit("info", "package.dry-run", item.PackageID, item.SourceID)
		return result
	}
	if err := m.installPackage(ctx, item, source, pkg, &result, options); err != nil {
		switch {
		case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
			result.Status, result.ErrorCode = StatusCancelled, CodeCancelled
		case errors.Is(err, ErrHashMismatch):
			result.Status, result.ErrorCode, result.Error = StatusFailed, CodeHashMismatch, "artifact checksum does not match the pinned SHA-256"
		case errors.Is(err, ErrSignatureInvalid):
			result.Status, result.ErrorCode, result.Error = StatusFailed, CodeSignatureFailed, "artifact signature is not trusted"
		default:
			if result.Status == "" {
				result.Status = StatusFailed
			}
			if result.ErrorCode == "" {
				result.ErrorCode = CodeInstallFailed
			}
			if result.Error == "" {
				result.Error = "installation failed"
			}
		}
		result.FinishedAt = time.Now().UTC()
		return result
	}
	// Verify real state after any operation that reached the system.
	after, err := Detect(ctx, m.Facts, pkg)
	if after.Installed {
		result.InstalledAfter = after.Version
	}
	if result.RebootRequired {
		// A reboot-required installer cannot be fully verified until Windows
		// restarts; the recorded status stays explicit and resumable.
		result.Status = StatusRebootRequired
		m.emit("info", "package.reboot-required", item.PackageID, detailVersion(after.Version))
		result.FinishedAt = time.Now().UTC()
		return result
	}
	if err == nil && !after.Installed {
		result.Status, result.ErrorCode, result.Error = StatusFailed, CodeVerifyFailed, "installed package was not detected after installation"
		m.emit("error", "package.verify-failed", item.PackageID, "")
	} else if result.Status == StatusSuccess && after.Installed {
		m.emit("info", "package.verified", item.PackageID, detailVersion(after.Version))
	}
	result.FinishedAt = time.Now().UTC()
	return result
}

func (m *Manager) installPackage(ctx context.Context, item PlanItem, source Source, pkg Package, result *Result, options InstallOptions) error {
	switch source.Kind {
	case "winget":
		return m.installWinget(ctx, item, source, result, options)
	case "http", "file":
		artifact, err := m.resolveArtifact(ctx, item, source, pkg, result, options)
		if err != nil {
			return err
		}
		if source.Type == "portable" {
			return m.installPortable(ctx, item, source, pkg, artifact, result)
		}
		return m.runInstaller(ctx, item, source, artifact, result, options)
	default:
		result.ErrorCode = CodeUnsupported
		return fmt.Errorf("unsupported source kind")
	}
}

func (m *Manager) installWinget(ctx context.Context, item PlanItem, source Source, result *Result, options InstallOptions) error {
	if m.Runner == nil || m.Winget == nil {
		result.ErrorCode = CodeSourceUnavailable
		return fmt.Errorf("winget is unavailable")
	}
	available, _ := m.Winget.Available(ctx)
	if !available {
		result.ErrorCode = CodeSourceUnavailable
		m.emit("warning", "package.winget-unavailable", item.PackageID, "")
		return fmt.Errorf("winget is unavailable")
	}
	subcommand := "install"
	if item.Action == ActionUpdate {
		subcommand = "upgrade"
	}
	args := []string{subcommand, "--id", source.WingetID, "--exact", "--silent", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}
	args = append(args, source.SilentArgs...)
	m.emit("info", "package.install-start", item.PackageID, "winget")
	timeout := options.InstallTimeout
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	execution, err := m.Runner.Run(runCtx, Command{Path: "winget", Args: args}, nil)
	if err != nil {
		if ctx.Err() != nil {
			result.ErrorCode = CodeCancelled
			return ctx.Err()
		}
		result.ErrorCode = CodeInstallFailed
		return err
	}
	result.Attempts++
	result.ExitCode = execution.ExitCode
	switch classifyExit(source, execution.ExitCode) {
	case exitSuccess:
		result.Status = StatusSuccess
		m.emit("info", "package.install-success", item.PackageID, "winget")
		return nil
	case exitReboot:
		result.Status, result.RebootRequired = StatusSuccess, true
		m.emit("info", "package.reboot-required", item.PackageID, "")
		return nil
	default:
		result.ErrorCode = CodeInstallFailed
		return fmt.Errorf("winget exited with code %d", execution.ExitCode)
	}
}

func (m *Manager) resolveArtifact(ctx context.Context, item PlanItem, source Source, pkg Package, result *Result, options InstallOptions) (string, error) {
	if source.Kind == "file" {
		path := m.expandPath(source.Path)
		info, err := m.Facts.File(ctx, path)
		if err != nil || !info.Exists {
			result.ErrorCode = CodeSourceUnavailable
			return "", fmt.Errorf("local installation file is unavailable")
		}
		if err := m.verifyArtifact(ctx, item, source, path); err != nil {
			return "", err
		}
		return path, nil
	}
	if m.CacheDir == "" {
		result.ErrorCode = CodeDownloadFailed
		return "", fmt.Errorf("download cache directory is required")
	}
	hash := normalizeHash(source.SHA256)
	name := artifactName(pkg.ID, source)
	destination := ""
	if hash != "" {
		destination = filepath.Join(m.CacheDir, "sha256", hash+filepath.Ext(name))
	} else {
		destination = filepath.Join(m.CacheDir, "artifacts", pkg.ID, name)
	}
	if hash != "" {
		if info, err := os.Stat(destination); err == nil && info.Mode().IsRegular() {
			if sum, sumErr := hashFile(destination); sumErr == nil && strings.EqualFold(sum, hash) {
				m.emit("info", "package.cache-hit", item.PackageID, "")
				result.ArtifactSHA256 = sum
				if err := m.verifySignature(ctx, item, source, destination); err != nil {
					return "", err
				}
				return destination, nil
			}
			os.Remove(destination)
		}
	}
	if m.Fetcher == nil {
		result.ErrorCode = CodeDownloadFailed
		return "", fmt.Errorf("downloader is required")
	}
	timeout := options.DownloadTimeout
	if timeout <= 0 {
		timeout = 45 * time.Minute
	}
	maxAttempts := options.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		downloadCtx, cancel := context.WithTimeout(ctx, timeout)
		m.emit("info", "package.download-start", item.PackageID, source.ID)
		downloadResult, err := m.Fetcher.Download(downloadCtx, DownloadRequest{URL: source.URL, Destination: destination, ExpectedSHA256: hash})
		cancel()
		if err == nil {
			m.emit("info", "package.download-complete", item.PackageID, formatSize(downloadResult.Size))
			result.ArtifactSHA256 = downloadResult.SHA256
			if err := m.verifySignature(ctx, item, source, downloadResult.Path); err != nil {
				return "", err
			}
			return downloadResult.Path, nil
		}
		lastErr = err
		m.emit("warning", "package.download-failed", item.PackageID, fmt.Sprintf("attempt %d", attempt))
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if errors.Is(err, ErrHashMismatch) {
			break
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(downloadRetryDelay(options.RetryDelay, attempt)):
		}
	}
	result.ErrorCode = CodeDownloadFailed
	if errors.Is(lastErr, ErrHashMismatch) {
		result.ErrorCode = CodeHashMismatch
		return "", ErrHashMismatch
	}
	return "", fmt.Errorf("download failed after %d attempts", maxAttempts)
}

func (m *Manager) verifyArtifact(ctx context.Context, item PlanItem, source Source, path string) error {
	if source.SHA256 != "" {
		sum, err := hashFile(path)
		if err != nil {
			return err
		}
		if !strings.EqualFold(sum, normalizeHash(source.SHA256)) {
			m.emit("error", "package.hash-mismatch", item.PackageID, "")
			return ErrHashMismatch
		}
		m.emit("info", "package.hash-verified", item.PackageID, "")
	}
	return m.verifySignature(ctx, item, source, path)
}

func (m *Manager) verifySignature(ctx context.Context, item PlanItem, source Source, path string) error {
	required := source.Signer != "" || source.Integrity == "signature"
	if !required {
		return nil
	}
	if m.Signer == nil {
		m.emit("warning", "package.signature-unavailable", item.PackageID, "")
		if source.Integrity == "signature" {
			return fmt.Errorf("%w: signature verification is required but unavailable", ErrSignatureInvalid)
		}
		return nil
	}
	signature, err := m.Signer.Verify(ctx, path)
	if err != nil || !signature.Verified {
		m.emit("error", "package.signature-failed", item.PackageID, "")
		return fmt.Errorf("%w: authenticode signature is not trusted", ErrSignatureInvalid)
	}
	if source.Signer != "" {
		haystack := strings.ToLower(signature.Subject + " " + signature.Signer)
		if !strings.Contains(haystack, strings.ToLower(source.Signer)) {
			m.emit("error", "package.publisher-mismatch", item.PackageID, "")
			return fmt.Errorf("%w: signature publisher does not match the catalog", ErrSignatureInvalid)
		}
	}
	m.emit("info", "package.signature-verified", item.PackageID, signature.Signer)
	return nil
}

func (m *Manager) runInstaller(ctx context.Context, item PlanItem, source Source, artifact string, result *Result, options InstallOptions) error {
	if m.Runner == nil {
		result.ErrorCode = CodeInstallFailed
		return fmt.Errorf("process runner is required")
	}
	command := installerCommand(source, artifact)
	timeout := options.InstallTimeout
	if timeout <= 0 {
		timeout = 30 * time.Minute
	}
	m.emit("info", "package.install-start", item.PackageID, source.Type)
	for attempt := 1; attempt <= 2; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		runCtx, cancel := context.WithTimeout(ctx, timeout)
		execution, err := m.Runner.Run(runCtx, command, nil)
		cancel()
		if err != nil {
			if ctx.Err() != nil {
				result.ErrorCode = CodeCancelled
				return ctx.Err()
			}
			result.ErrorCode = CodeInstallFailed
			return err
		}
		result.Attempts++
		result.ExitCode = execution.ExitCode
		switch classifyExit(source, execution.ExitCode) {
		case exitSuccess:
			result.Status = StatusSuccess
			m.emit("info", "package.install-success", item.PackageID, "")
			return nil
		case exitReboot:
			result.Status, result.RebootRequired = StatusSuccess, true
			m.emit("info", "package.reboot-required", item.PackageID, "")
			return nil
		case exitRetry:
			if attempt == 1 {
				m.emit("warning", "package.install-retry", item.PackageID, fmt.Sprintf("%d", execution.ExitCode))
				delay := options.RetryDelay
				if delay <= 0 {
					delay = 5 * time.Second
				}
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}
				continue
			}
			result.ErrorCode = CodeInstallFailed
			return fmt.Errorf("installer exited with retryable code %d", execution.ExitCode)
		default:
			result.ErrorCode = CodeInstallFailed
			return fmt.Errorf("installer exited with code %d", execution.ExitCode)
		}
	}
	return fmt.Errorf("installer did not complete")
}

func (m *Manager) installPortable(ctx context.Context, item PlanItem, source Source, pkg Package, artifact string, result *Result) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	dir := m.expandPath(source.PortableDir)
	if strings.TrimSpace(dir) == "" {
		dir = filepath.Join(os.Getenv("ProgramFiles"), "WinDeploy", pkg.ID)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		result.ErrorCode = CodeInstallFailed
		return fmt.Errorf("could not create portable application directory")
	}
	target := filepath.Join(dir, filepath.Base(artifact))
	input, err := os.Open(artifact)
	if err != nil {
		result.ErrorCode = CodeInstallFailed
		return fmt.Errorf("could not open portable artifact")
	}
	defer input.Close()
	temp, err := os.CreateTemp(dir, ".wd-portable-*")
	if err != nil {
		result.ErrorCode = CodeInstallFailed
		return fmt.Errorf("could not create portable target file")
	}
	tempName := temp.Name()
	fail := func(message string) error {
		temp.Close()
		os.Remove(tempName)
		result.ErrorCode = CodeInstallFailed
		return fmt.Errorf("%s", message)
	}
	if _, err := io.Copy(temp, input); err != nil {
		return fail("could not copy portable application")
	}
	if err := temp.Sync(); err != nil {
		return fail("could not flush portable application")
	}
	if err := temp.Close(); err != nil {
		os.Remove(tempName)
		result.ErrorCode = CodeInstallFailed
		return fmt.Errorf("could not close portable application")
	}
	if err := os.Rename(tempName, target); err != nil {
		os.Remove(tempName)
		result.ErrorCode = CodeInstallFailed
		return fmt.Errorf("could not store portable application")
	}
	result.Status = StatusSuccess
	m.emit("info", "package.portable-installed", item.PackageID, filepath.Base(target))
	return nil
}

type exitClass int

const (
	exitSuccess exitClass = iota
	exitReboot
	exitRetry
	exitFailure
)

func classifyExit(source Source, code int) exitClass {
	if code == 0 {
		return exitSuccess
	}
	if code == 3010 || code == 1641 {
		return exitReboot
	}
	for _, allowed := range source.SuccessCodes {
		if code == allowed {
			return exitSuccess
		}
	}
	for _, retry := range source.RetryCodes {
		if code == retry {
			return exitRetry
		}
	}
	if source.Type == "msi" && code == 1618 {
		return exitRetry
	}
	return exitFailure
}

func installerCommand(source Source, artifact string) Command {
	if source.Type == "msi" {
		args := []string{"/i", artifact, "/qn", "/norestart"}
		if len(source.SilentArgs) > 0 {
			args = append([]string{"/i", artifact}, source.SilentArgs...)
		}
		return Command{Path: "msiexec.exe", Args: args}
	}
	return Command{Path: artifact, Args: append([]string{}, source.SilentArgs...)}
}

func findSource(pkg Package, id string) (Source, bool) {
	for _, source := range pkg.Sources {
		if source.ID == id {
			return source, true
		}
	}
	return Source{}, false
}

func (m *Manager) expandPath(path string) string {
	if m.Facts == nil {
		return path
	}
	return m.Facts.ExpandPath(path)
}

func artifactName(packageID string, source Source) string {
	if source.FileName != "" {
		return filepath.Base(source.FileName)
	}
	if source.URL != "" {
		if parsed, err := url.Parse(source.URL); err == nil {
			name := filepath.Base(parsed.Path)
			if name != "" && name != "." && name != "/" && !strings.ContainsAny(name, "\\:*?\"<>|") {
				return name
			}
		}
	}
	extension := ".exe"
	if source.Type == "msi" {
		extension = ".msi"
	}
	return packageID + extension
}

func detailVersion(version string) string {
	if version == "" {
		return "version-unknown"
	}
	return version
}

func formatSize(size int64) string {
	if size <= 0 {
		return "0 B"
	}
	if size < 1024 {
		return fmt.Sprintf("%d B", size)
	}
	value := float64(size)
	for _, unit := range []string{"KB", "MB", "GB", "TB"} {
		value /= 1024
		if value < 1024 || unit == "TB" {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
	}
	return fmt.Sprintf("%d B", size)
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", err
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func downloadRetryDelay(configured time.Duration, attempt int) time.Duration {
	if configured > 0 {
		return configured
	}
	return time.Duration(attempt) * 2 * time.Second
}
