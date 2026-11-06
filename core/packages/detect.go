package packages

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"windeploy/core/inventory"
)

// FileInfo is the read-only evidence for file-based detection. Version may be
// empty when a file has no version resource.
type FileInfo struct {
	Exists  bool
	Version string
}

// Facts supplies detection evidence. Implementations must be read-only and honor
// context where the underlying Windows API allows it. ExpandPath resolves
// %VARIABLE% and drive-relative catalog paths for the current user.
type Facts interface {
	Apps(context.Context) ([]inventory.InstalledApp, error)
	File(context.Context, string) (FileInfo, error)
	ExpandPath(string) string
}

// InstalledState is the detection outcome for one package. Installed=false with a
// non-nil error is unknown, never proof of absence.
type InstalledState struct {
	Installed bool   `json:"installed"`
	Version   string `json:"version,omitempty"`
	Evidence  string `json:"evidence,omitempty"`
	Completed bool   `json:"completed"`
}

type compiledDetection struct {
	detection Detection
	name      *regexp.Regexp
	publisher *regexp.Regexp
	product   string
}

// compileDetections precompiles patterns once per plan/install run.
func compileDetections(pkg Package) ([]compiledDetection, error) {
	result := make([]compiledDetection, 0, len(pkg.Detections))
	for _, d := range pkg.Detections {
		item := compiledDetection{detection: d}
		if d.NamePattern != "" {
			re, err := regexp.Compile("(?i)" + d.NamePattern)
			if err != nil {
				return nil, fmt.Errorf("invalid detection pattern")
			}
			item.name = re
		}
		if d.PublisherPattern != "" {
			re, err := regexp.Compile("(?i)" + d.PublisherPattern)
			if err != nil {
				return nil, fmt.Errorf("invalid publisher pattern")
			}
			item.publisher = re
		}
		if d.Kind == "msi" {
			product, err := parseGUID(d.ProductCode)
			if err != nil {
				return nil, err
			}
			item.product = product
		}
		result = append(result, item)
	}
	return result, nil
}

// Detect evaluates all rules for a package. A rule that matches establishes the
// package as installed; the highest detected version wins. Registry read failures
// are returned so callers can distinguish "absent" from "unknown".
func Detect(ctx context.Context, facts Facts, pkg Package) (InstalledState, error) {
	if facts == nil {
		return InstalledState{}, fmt.Errorf("facts provider is required")
	}
	compiled, err := compileDetections(pkg)
	if err != nil {
		return InstalledState{}, err
	}
	needsApps := false
	for _, item := range compiled {
		if item.detection.Kind != "file" {
			needsApps = true
			break
		}
	}
	var apps []inventory.InstalledApp
	var appsErr error
	if needsApps {
		if err := ctx.Err(); err != nil {
			return InstalledState{}, err
		}
		apps, appsErr = facts.Apps(ctx)
	}
	state := InstalledState{Completed: true}
	var firstErr error
	for _, item := range compiled {
		switch item.detection.Kind {
		case "uninstall", "msi":
			if appsErr != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("installed programs are unavailable")
				}
				continue
			}
			for _, app := range apps {
				if !matches(item, app) {
					continue
				}
				if state.Version == "" || CompareVersions(app.Version, state.Version) > 0 {
					state.Version = strings.TrimSpace(app.Version)
				}
				state.Installed = true
				state.Evidence = app.Name
				break
			}
		case "file":
			info, fileErr := facts.File(ctx, item.detection.Path)
			if fileErr != nil {
				if firstErr == nil {
					firstErr = fmt.Errorf("file evidence is unavailable")
				}
				continue
			}
			if !info.Exists {
				continue
			}
			state.Installed = true
			if state.Evidence == "" {
				state.Evidence = item.detection.Path
			}
			if info.Version != "" && (state.Version == "" || CompareVersions(info.Version, state.Version) > 0) {
				state.Version = strings.TrimSpace(info.Version)
			}
		}
	}
	if state.Installed {
		return state, nil
	}
	return state, firstErr
}

func matches(item compiledDetection, app inventory.InstalledApp) bool {
	switch item.detection.Kind {
	case "uninstall":
		if item.name == nil || !item.name.MatchString(app.Name) {
			return false
		}
		if item.publisher != nil && !item.publisher.MatchString(app.Publisher) {
			return false
		}
		return true
	case "msi":
		if !app.WindowsInstaller {
			return false
		}
		key := strings.ToUpper(strings.Trim(strings.TrimSpace(app.Key), "{}"))
		return key == item.product
	default:
		return false
	}
}
