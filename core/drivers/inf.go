package drivers

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"
)

// MaxINFBytes bounds one INF file.
const MaxINFBytes = 1 << 20

// INFInfo is the catalog-relevant subset of an INF file.
type INFInfo struct {
	Path        string   `json:"path"`
	Version     string   `json:"version,omitempty"`
	Provider    string   `json:"provider,omitempty"`
	Class       string   `json:"class,omitempty"`
	HardwareIDs []string `json:"hardware_ids"`
}

// InspectINF parses one INF file.
func InspectINF(path string) (INFInfo, error) {
	file, err := os.Open(path)
	if err != nil {
		return INFInfo{}, fmt.Errorf("INF file is unavailable")
	}
	defer file.Close()
	info, err := ParseINF(file)
	if err != nil {
		return INFInfo{}, err
	}
	info.Path = path
	return info, nil
}

// ParseINF parses a bounded INF stream. UTF-16LE (with or without BOM) and UTF-8
// are supported. DriverVer and %Strings% expansion are handled; driver logic is
// never executed.
func ParseINF(reader io.Reader) (INFInfo, error) {
	data, err := io.ReadAll(io.LimitReader(reader, MaxINFBytes+1))
	if err != nil {
		return INFInfo{}, fmt.Errorf("INF file could not be read")
	}
	if len(data) > MaxINFBytes {
		return INFInfo{}, fmt.Errorf("INF file exceeds the size limit")
	}
	text := decodeINF(data)
	sections := parseSections(text)
	info := INFInfo{HardwareIDs: []string{}}
	stringsTable := parseKeyValues(sections["strings"])
	for key, value := range stringsTable {
		stringsTable[key] = unquoteINF(value)
	}
	expand := func(value string) string {
		for i := 0; i < 8; i++ {
			next := expandStrings(value, stringsTable)
			if next == value {
				break
			}
			value = next
		}
		return value
	}
	if values, ok := sections["version"]; ok {
		keys := parseKeyValues(values)
		info.Provider = expand(keys["provider"])
		info.Class = expand(keys["class"])
		if driverVer := keys["driverver"]; driverVer != "" {
			parts := strings.SplitN(driverVer, ",", 2)
			if len(parts) == 2 {
				info.Version = strings.TrimSpace(parts[1])
			}
		}
	}
	modelSections := map[string]bool{}
	if values, ok := sections["manufacturer"]; ok {
		for _, line := range splitLines(values) {
			key, value, ok := splitKeyValue(line)
			if !ok {
				continue
			}
			_ = key
			for _, part := range strings.Split(value, ",") {
				name := strings.TrimSpace(part)
				if name != "" {
					modelSections[strings.ToLower(name)] = true
				}
			}
		}
	}
	for name := range modelSections {
		values, ok := sections[name]
		if !ok {
			continue
		}
		info.HardwareIDs = append(info.HardwareIDs, hardwareIDsFromSection(values, expand)...)
	}
	if len(info.HardwareIDs) == 0 {
		for name, values := range sections {
			if name == "version" || name == "strings" || name == "manufacturer" || strings.HasPrefix(name, "source") || strings.HasPrefix(name, "destination") {
				continue
			}
			info.HardwareIDs = append(info.HardwareIDs, hardwareIDsFromSection(values, expand)...)
		}
	}
	info.HardwareIDs = uniqueStrings(info.HardwareIDs)
	if info.Version == "" && info.Provider == "" && len(info.HardwareIDs) == 0 {
		return INFInfo{}, fmt.Errorf("INF file does not look like a driver package")
	}
	return info, nil
}

// ScanDirectory finds valid INF packages under root (depth-limited, read-only).
func ScanDirectory(ctx context.Context, root string, maxFiles int) ([]INFInfo, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("driver directory is required")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("driver directory is unavailable")
	}
	if maxFiles <= 0 || maxFiles > 5000 {
		maxFiles = 2000
	}
	result := []INFInfo{}
	inspected := 0
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		relative, relErr := filepath.Rel(root, path)
		if relErr == nil && strings.Count(relative, string(os.PathSeparator)) > 3 {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".inf") {
			return nil
		}
		inspected++
		if inspected > maxFiles {
			return fmt.Errorf("driver directory contains too many INF files")
		}
		parsed, parseErr := InspectINF(path)
		if parseErr == nil {
			result = append(result, parsed)
		}
		return nil
	})
	if err != nil {
		return result, err
	}
	return result, nil
}

// FindForHardwareID scans a catalog directory and returns INFs matching a device
// hardware ID exactly (case-insensitive).
func FindForHardwareID(ctx context.Context, root, hardwareID string) ([]INFInfo, error) {
	hardwareID = strings.TrimSpace(hardwareID)
	if hardwareID == "" {
		return nil, fmt.Errorf("hardware ID is required")
	}
	packages, err := ScanDirectory(ctx, root, 2000)
	if err != nil {
		return nil, err
	}
	matches := []INFInfo{}
	for _, pkg := range packages {
		for _, id := range pkg.HardwareIDs {
			if strings.EqualFold(id, hardwareID) {
				matches = append(matches, pkg)
				break
			}
		}
	}
	return matches, nil
}

func decodeINF(data []byte) string {
	if len(data) >= 2 && data[0] == 0xFF && data[1] == 0xFE {
		return decodeUTF16(data[2:])
	}
	if len(data) >= 2 && data[0] == 0xFE && data[1] == 0xFF {
		units := make([]uint16, 0, (len(data)-2)/2)
		for i := 2; i+1 < len(data); i += 2 {
			units = append(units, uint16(data[i])<<8|uint16(data[i+1]))
		}
		return string(utf16.Decode(units))
	}
	nulBytes := 0
	for _, b := range data {
		if b == 0 {
			nulBytes++
		}
	}
	if len(data) > 16 && nulBytes > len(data)/4 {
		return decodeUTF16(data)
	}
	return string(data)
}

func decodeUTF16(data []byte) string {
	units := make([]uint16, 0, len(data)/2)
	for i := 0; i+1 < len(data); i += 2 {
		units = append(units, uint16(data[i])|uint16(data[i+1])<<8)
	}
	return string(utf16.Decode(units))
}

func parseSections(text string) map[string][]string {
	sections := map[string][]string{}
	current := ""
	for _, raw := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		line := strings.TrimSpace(raw)
		if index := strings.Index(line, ";"); index >= 0 {
			line = strings.TrimSpace(line[:index])
		}
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.ToLower(strings.TrimSpace(line[1 : len(line)-1]))
			if _, exists := sections[current]; !exists {
				sections[current] = []string{}
			}
			continue
		}
		if current != "" {
			sections[current] = append(sections[current], line)
		}
	}
	return sections
}

func parseKeyValues(lines []string) map[string]string {
	result := map[string]string{}
	for _, line := range splitLines(lines) {
		key, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		result[strings.ToLower(strings.TrimSpace(key))] = strings.TrimSpace(value)
	}
	return result
}

func splitKeyValue(line string) (string, string, bool) {
	index := strings.Index(line, "=")
	if index <= 0 {
		return "", "", false
	}
	return line[:index], line[index+1:], true
}

func splitLines(lines []string) []string {
	return lines
}

func expandStrings(value string, table map[string]string) string {
	for name, replacement := range table {
		token := "%" + name + "%"
		if strings.Contains(strings.ToLower(value), token) {
			// Case-insensitive replacement for the common INF convention.
			lower := strings.ToLower(value)
			index := strings.Index(lower, token)
			value = value[:index] + replacement + value[index+len(token):]
		}
	}
	return value
}

func hardwareIDsFromSection(lines []string, expand func(string) string) []string {
	result := []string{}
	for _, line := range lines {
		_, value, ok := splitKeyValue(line)
		if !ok {
			continue
		}
		parts := strings.Split(value, ",")
		for _, part := range parts[1:] {
			candidate := strings.TrimSpace(expand(part))
			candidate = strings.Trim(candidate, "\"")
			if isHardwareID(candidate) {
				result = append(result, candidate)
			}
		}
	}
	return result
}

func isHardwareID(value string) bool {
	if value == "" || len(value) > 300 {
		return false
	}
	upper := strings.ToUpper(value)
	prefixes := []string{"PCI\\", "USB\\", "HDAUDIO\\", "ACPI\\", "SCSI\\", "ROOT\\", "SW\\", "DISPLAY\\", "MONITOR\\", "VID_", "PID_"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	// Some INF entries reference another hardware ID indirectly (e.g. a device
	// class GUID); only backslash-containing values are treated as IDs.
	return strings.Contains(value, "\\")
}

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.ToLower(value)
		if value == "" || seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, value)
	}
	return result
}

func unquoteINF(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && value[0] == '"' && value[len(value)-1] == '"' {
		return value[1 : len(value)-1]
	}
	return value
}
