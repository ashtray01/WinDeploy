//go:build windows

package packages

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"windeploy/core/inventory"
)

// WindowsFacts reads uninstall registry entries, file version resources and
// environment variables. It never executes software.
type WindowsFacts struct {
	AppsProvider inventory.AppProvider
}

func (f WindowsFacts) apps() inventory.AppProvider {
	if f.AppsProvider != nil {
		return f.AppsProvider
	}
	return inventory.Native{}
}

// Apps enumerates visible uninstall entries in machine and current-user views.
func (f WindowsFacts) Apps(ctx context.Context) ([]inventory.InstalledApp, error) {
	return f.apps().Apps(ctx)
}

// File reports existence and the fixed file version of a local path. Wildcards
// (* and ?) are supported for versioned installation directories.
func (f WindowsFacts) File(ctx context.Context, path string) (FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return FileInfo{}, err
	}
	expanded := f.ExpandPath(path)
	if strings.TrimSpace(expanded) == "" {
		return FileInfo{}, fmt.Errorf("path is empty")
	}
	if strings.ContainsAny(expanded, "*?") {
		matches, globErr := filepath.Glob(expanded)
		if globErr != nil {
			return FileInfo{}, fmt.Errorf("path pattern is invalid")
		}
		for _, match := range matches {
			info, statErr := os.Stat(match)
			if statErr != nil || !info.Mode().IsRegular() {
				continue
			}
			version, _ := fileVersion(match)
			return FileInfo{Exists: true, Version: version}, nil
		}
		return FileInfo{Exists: false}, nil
	}
	info, err := os.Stat(expanded)
	if err != nil {
		if os.IsNotExist(err) {
			return FileInfo{Exists: false}, nil
		}
		return FileInfo{}, fmt.Errorf("file information is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return FileInfo{}, err
	}
	version, _ := fileVersion(expanded)
	return FileInfo{Exists: info.Mode().IsRegular(), Version: version}, nil
}

var percentVariable = regexp.MustCompile(`%([^%]+)%`)

// ExpandPath expands %VARIABLE% references and cleans the path. Unknown variables
// expand to an empty string, which keeps a missing catalog path from resolving to
// an unrelated location.
func (f WindowsFacts) ExpandPath(path string) string {
	expanded := percentVariable.ReplaceAllStringFunc(path, func(match string) string {
		name := strings.Trim(match, "%")
		if value, ok := os.LookupEnv(name); ok {
			return value
		}
		return ""
	})
	if expanded == path && strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			expanded = home + strings.TrimPrefix(path, "~")
		}
	}
	if strings.TrimSpace(expanded) == "" {
		return expanded
	}
	return filepath.Clean(expanded)
}

func fileVersion(path string) (string, error) {
	size, err := windows.GetFileVersionInfoSize(path, nil)
	if err != nil || size == 0 || size > 1<<20 {
		return "", fmt.Errorf("version resource is unavailable")
	}
	buffer := make([]byte, size)
	if err := windows.GetFileVersionInfo(path, 0, size, unsafe.Pointer(&buffer[0])); err != nil {
		return "", fmt.Errorf("version resource is unavailable")
	}
	var info *windows.VS_FIXEDFILEINFO
	var length uint32
	if err := windows.VerQueryValue(unsafe.Pointer(&buffer[0]), `\`, unsafe.Pointer(&info), &length); err != nil || info == nil || length < uint32(unsafe.Sizeof(windows.VS_FIXEDFILEINFO{})) {
		return "", fmt.Errorf("version resource is unavailable")
	}
	if info.Signature != 0xFEEF04BD {
		return "", fmt.Errorf("version resource is invalid")
	}
	parts := []uint16{
		uint16(info.FileVersionMS >> 16), uint16(info.FileVersionMS & 0xffff),
		uint16(info.FileVersionLS >> 16), uint16(info.FileVersionLS & 0xffff),
	}
	return fmt.Sprintf("%d.%d.%d.%d", parts[0], parts[1], parts[2], parts[3]), nil
}
