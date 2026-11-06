package packages

import (
	"context"
	"fmt"
	"os"
	"time"
)

// ArtifactResult is the outcome of preparing one installer without running it.
type ArtifactResult struct {
	PackageID  string `json:"package_id"`
	Name       string `json:"name"`
	SourceID   string `json:"source_id,omitempty"`
	SourceKind string `json:"source_kind,omitempty"`
	Status     Status `json:"status"`
	Path       string `json:"path,omitempty"`
	SHA256     string `json:"sha256,omitempty"`
	Size       int64  `json:"size,omitempty"`
	Cached     bool   `json:"cached"`
	ErrorCode  string `json:"error_code,omitempty"`
	Error      string `json:"error,omitempty"`
}

// DownloadRun is the outcome of preparing one or more artifacts.
type DownloadRun struct {
	StartedAt  time.Time        `json:"started_at"`
	FinishedAt time.Time        `json:"finished_at"`
	Results    []ArtifactResult `json:"results"`
}

// Download resolves and verifies installers into the cache without installing
// anything. WinGet-managed packages are skipped because WinGet performs its own
// hash-verified download during installation.
func (m *Manager) Download(ctx context.Context, selection []string, options InstallOptions) (DownloadRun, error) {
	if m.Facts == nil {
		return DownloadRun{}, fmt.Errorf("facts provider is required")
	}
	if err := m.Catalog.Validate(); err != nil {
		return DownloadRun{}, err
	}
	planCtx, stopPlanning := context.WithTimeout(context.Background(), 2*time.Minute)
	plan, err := (Planner{Catalog: m.Catalog, Facts: m.Facts, System: m.System, Winget: m.Winget}).Build(planCtx, selection)
	stopPlanning()
	if err != nil {
		return DownloadRun{}, err
	}
	run := DownloadRun{StartedAt: time.Now().UTC(), Results: []ArtifactResult{}}
	for _, item := range plan.Items {
		result := ArtifactResult{PackageID: item.PackageID, Name: item.Name, SourceID: item.SourceID, SourceKind: item.SourceKind}
		if err := ctx.Err(); err != nil {
			result.Status, result.ErrorCode = StatusCancelled, CodeCancelled
			run.Results = append(run.Results, result)
			continue
		}
		if item.Action == ActionUpToDate {
			result.Status, result.ErrorCode = StatusSkipped, "already-installed"
			run.Results = append(run.Results, result)
			continue
		}
		if !item.Installable() {
			result.Status, result.ErrorCode = StatusUnavailable, item.ReasonCode
			run.Results = append(run.Results, result)
			continue
		}
		pkg, ok := m.Catalog.Find(item.PackageID)
		if !ok {
			result.Status, result.ErrorCode = StatusUnknown, CodeUnknownSource
			run.Results = append(run.Results, result)
			continue
		}
		source, ok := findSource(pkg, item.SourceID)
		if !ok {
			result.Status, result.ErrorCode = StatusUnknown, CodeUnknownSource
			run.Results = append(run.Results, result)
			continue
		}
		if source.Kind == "winget" {
			result.Status, result.ErrorCode = StatusSkipped, "winget-managed"
			run.Results = append(run.Results, result)
			continue
		}
		installResult := Result{PackageID: item.PackageID, Name: item.Name, Action: item.Action, SourceID: item.SourceID, SourceVersion: item.SourceVersion}
		destination := artifactDestination(m.CacheDir, pkg.ID, source)
		wasCached := false
		if destination != "" {
			if info, statErr := os.Stat(destination); statErr == nil && info.Mode().IsRegular() {
				wasCached = true
			}
		}
		path, err := m.resolveArtifact(ctx, item, source, pkg, &installResult, options)
		if err != nil {
			result.Status, result.ErrorCode = StatusFailed, installResult.ErrorCode
			if result.ErrorCode == "" {
				result.ErrorCode = CodeDownloadFailed
			}
			result.Error = "artifact could not be prepared"
			run.Results = append(run.Results, result)
			continue
		}
		digest, hashErr := hashFile(path)
		if hashErr != nil {
			result.Status, result.ErrorCode = StatusFailed, CodeDownloadFailed
			result.Error = "artifact could not be hashed"
			run.Results = append(run.Results, result)
			continue
		}
		size := int64(0)
		if info, statErr := os.Stat(path); statErr == nil {
			size = info.Size()
		}
		result.Status, result.Path, result.SHA256, result.Size = StatusSuccess, path, digest, size
		result.Cached = wasCached
		run.Results = append(run.Results, result)
	}
	run.FinishedAt = time.Now().UTC()
	return run, nil
}
