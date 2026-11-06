// Package packages installs workstation software from a validated catalog.
//
// The package is intentionally independent from the UI and from concrete Windows
// operations: detection, downloading, signature verification and process execution
// are injected adapters. The same domain model is used by the Modern GUI, the
// legacy GUI and headless runs in a test VM.
package packages

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"windeploy/core/engine"
)

// CatalogVersion is the only supported YAML catalog schema.
const CatalogVersion = 1

// Action is the planned outcome for one catalog package.
type Action string

const (
	ActionInstall      Action = "install"
	ActionUpdate       Action = "update"
	ActionUpToDate     Action = "up-to-date"
	ActionUnavailable  Action = "unavailable"
	ActionIncompatible Action = "incompatible"
	ActionUnknown      Action = "unknown"
)

// Valid reports whether a is a recognized action.
func (a Action) Valid() bool {
	switch a {
	case ActionInstall, ActionUpdate, ActionUpToDate, ActionUnavailable, ActionIncompatible, ActionUnknown:
		return true
	default:
		return false
	}
}

// Package describes one installable program independently of its sources.
type Package struct {
	ID           string                `yaml:"id" json:"id"`
	Name         string                `yaml:"name" json:"name"`
	Publisher    string                `yaml:"publisher" json:"publisher"`
	Category     string                `yaml:"category" json:"category"`
	Group        string                `yaml:"group,omitempty" json:"group,omitempty"`
	Description  string                `yaml:"description,omitempty" json:"description,omitempty"`
	Requirements engine.OSRequirements `yaml:"requirements,omitempty" json:"requirements"`
	Detections   []Detection           `yaml:"detection" json:"detection"`
	Sources      []Source              `yaml:"sources" json:"sources"`
	// RequiresAdmin marks installers that need an elevated token. Detection and
	// planning do not need elevation.
	RequiresAdmin bool `yaml:"requires_admin,omitempty" json:"requires_admin"`
}

// Detection describes read-only evidence that a package is installed.
type Detection struct {
	// Kind is one of "uninstall", "file" or "msi".
	Kind string `yaml:"kind" json:"kind"`
	// NamePattern is a case-insensitive regular expression matched against the
	// uninstall DisplayName.
	NamePattern string `yaml:"name_pattern,omitempty" json:"name_pattern,omitempty"`
	// PublisherPattern optionally narrows uninstall entries by Publisher.
	PublisherPattern string `yaml:"publisher_pattern,omitempty" json:"publisher_pattern,omitempty"`
	// ProductCode is an MSI product GUID used for "msi" detection.
	ProductCode string `yaml:"product_code,omitempty" json:"product_code,omitempty"`
	// Path is an environment-expanded file path used for "file" detection.
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
}

// Source is one concrete installation route. The planner picks the first source
// that is applicable, available and allowed by integrity policy.
type Source struct {
	ID         string   `yaml:"id" json:"id"`
	Kind       string   `yaml:"kind" json:"kind"` // winget, http, file
	WingetID   string   `yaml:"winget_id,omitempty" json:"winget_id,omitempty"`
	URL        string   `yaml:"url,omitempty" json:"url,omitempty"`
	Path       string   `yaml:"path,omitempty" json:"path,omitempty"`
	Version    string   `yaml:"version,omitempty" json:"version,omitempty"`
	Arch       string   `yaml:"arch,omitempty" json:"arch,omitempty"`
	SHA256     string   `yaml:"sha256,omitempty" json:"sha256,omitempty"`
	Integrity  string   `yaml:"integrity,omitempty" json:"integrity,omitempty"` // sha256 (default) or signature
	Type       string   `yaml:"type,omitempty" json:"type,omitempty"`           // exe, msi, portable, winget
	SilentArgs []string `yaml:"silent_args,omitempty" json:"silent_args,omitempty"`
	// SuccessCodes overrides the default success exit codes. 0 is always accepted;
	// 3010/1641 (reboot required/initiated) are treated as success plus reboot.
	SuccessCodes []int `yaml:"success_codes,omitempty" json:"success_codes,omitempty"`
	// RetryCodes are exit codes that are safe to retry (for example 1618, another
	// MSI installation in progress).
	RetryCodes []int `yaml:"retry_codes,omitempty" json:"retry_codes,omitempty"`
	// Signer is an expected Authenticode subject fragment (CN or full name).
	Signer string `yaml:"signer,omitempty" json:"signer,omitempty"`
	// Priority orders sources; lower values are preferred. Equal priorities keep
	// catalog order.
	Priority int `yaml:"priority,omitempty" json:"priority"`
	// SizeBytes is advisory for plan display; it is not trusted for space checks.
	SizeBytes int64 `yaml:"size_bytes,omitempty" json:"size_bytes,omitempty"`
	// PortableDir is the destination directory for Type "portable". It supports
	// %ENV% expansion and defaults to %ProgramFiles%\WinDeploy\<package ID>.
	PortableDir string `yaml:"portable_dir,omitempty" json:"portable_dir,omitempty"`
	// FileName overrides the downloaded artifact name.
	FileName string `yaml:"file_name,omitempty" json:"file_name,omitempty"`
}

// Catalog is a validated set of packages.
type Catalog struct {
	Version  int       `yaml:"version" json:"version"`
	Packages []Package `yaml:"packages" json:"packages"`
}

var packageID = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,63}$`)
var sha256Hex = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validHexHash(value string) bool {
	return sha256Hex.MatchString(strings.ToLower(strings.TrimSpace(value)))
}

func normalizeHash(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

// HashBytes returns the lowercase SHA-256 hex digest of data.
func HashBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Validate checks catalog invariants without touching the target system.
func (c Catalog) Validate() error {
	if c.Version != CatalogVersion {
		return fmt.Errorf("unsupported package catalog version")
	}
	if len(c.Packages) == 0 {
		return fmt.Errorf("package catalog is empty")
	}
	if len(c.Packages) > 256 {
		return fmt.Errorf("package catalog exceeds entry limit")
	}
	seen := map[string]bool{}
	for i, p := range c.Packages {
		if err := p.Validate(); err != nil {
			return fmt.Errorf("package at index %d: %w", i, err)
		}
		if seen[p.ID] {
			return fmt.Errorf("duplicate package ID %q", p.ID)
		}
		seen[p.ID] = true
	}
	return nil
}

// PackagesByCategory returns packages ordered by display category while keeping
// catalog order inside one category. It returns a copy.
func (c Catalog) PackagesByCategory() []Package {
	order := []string{"browsers", "messengers", "media", "utilities", "printing", "office", "other"}
	rank := map[string]int{}
	for i, name := range order {
		rank[name] = i
	}
	result := append([]Package{}, c.Packages...)
	sort.SliceStable(result, func(i, j int) bool {
		left, ok := rank[result[i].Category]
		if !ok {
			left = len(order)
		}
		right, ok := rank[result[j].Category]
		if !ok {
			right = len(order)
		}
		return left < right
	})
	return result
}

// Validate checks one package and its sources.
func (p Package) Validate() error {
	if !packageID.MatchString(p.ID) {
		return fmt.Errorf("invalid package ID")
	}
	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Publisher) == "" || strings.TrimSpace(p.Category) == "" {
		return fmt.Errorf("package %q requires name, publisher and category", p.ID)
	}
	switch p.Category {
	case "browsers", "messengers", "media", "utilities", "printing", "office", "other":
	default:
		return fmt.Errorf("package %q has unsupported category", p.ID)
	}
	for _, version := range p.Requirements.Versions {
		switch version {
		case "windows-7-sp1", "windows-10", "windows-11":
		default:
			return fmt.Errorf("package %q has unsupported Windows requirement", p.ID)
		}
	}
	for _, arch := range p.Requirements.Architectures {
		if arch != "386" && arch != "amd64" {
			return fmt.Errorf("package %q has unsupported architecture requirement", p.ID)
		}
	}
	if len(p.Detections) == 0 {
		return fmt.Errorf("package %q requires at least one detection rule", p.ID)
	}
	for i, d := range p.Detections {
		if err := d.validate(); err != nil {
			return fmt.Errorf("package %q detection %d: %w", p.ID, i, err)
		}
	}
	if len(p.Sources) == 0 {
		return fmt.Errorf("package %q requires at least one source", p.ID)
	}
	sourceIDs := map[string]bool{}
	for i, s := range p.Sources {
		if err := s.validate(); err != nil {
			return fmt.Errorf("package %q source %d: %w", p.ID, i, err)
		}
		if sourceIDs[s.ID] {
			return fmt.Errorf("package %q has duplicate source ID", p.ID)
		}
		sourceIDs[s.ID] = true
	}
	return nil
}

func (d Detection) validate() error {
	switch d.Kind {
	case "uninstall":
		if strings.TrimSpace(d.NamePattern) == "" || d.ProductCode != "" || d.Path != "" {
			return fmt.Errorf("uninstall detection requires only name_pattern")
		}
		if _, err := regexp.Compile(d.NamePattern); err != nil {
			return fmt.Errorf("invalid name_pattern")
		}
		if d.PublisherPattern != "" {
			if _, err := regexp.Compile(d.PublisherPattern); err != nil {
				return fmt.Errorf("invalid publisher_pattern")
			}
		}
	case "msi":
		if _, err := parseGUID(d.ProductCode); err != nil {
			return fmt.Errorf("invalid MSI product code: %w", err)
		}
		if d.NamePattern != "" || d.PublisherPattern != "" || d.Path != "" {
			return fmt.Errorf("msi detection requires only product_code")
		}
	case "file":
		if strings.TrimSpace(d.Path) == "" || d.NamePattern != "" || d.ProductCode != "" {
			return fmt.Errorf("file detection requires only path")
		}
	default:
		return fmt.Errorf("unsupported detection kind")
	}
	return nil
}

func (s Source) validate() error {
	if !packageID.MatchString(s.ID) {
		return fmt.Errorf("invalid source ID")
	}
	if s.Arch != "" && s.Arch != "386" && s.Arch != "amd64" {
		return fmt.Errorf("unsupported source architecture")
	}
	if s.Version != "" && !versionLike(s.Version) {
		return fmt.Errorf("invalid source version")
	}
	if s.SizeBytes < 0 {
		return fmt.Errorf("negative source size")
	}
	for _, arg := range s.SilentArgs {
		if strings.ContainsAny(arg, "\x00\r\n") {
			return fmt.Errorf("invalid silent argument")
		}
	}
	switch s.Kind {
	case "winget":
		if strings.TrimSpace(s.WingetID) == "" || s.URL != "" || s.Path != "" || s.SHA256 != "" {
			return fmt.Errorf("winget source requires only winget_id")
		}
		sType := s.Type
		if sType == "" {
			sType = "winget"
		}
		if sType != "winget" {
			return fmt.Errorf("winget source has invalid type")
		}
	case "http":
		if s.URL == "" || s.Path != "" || s.WingetID != "" {
			return fmt.Errorf("http source requires url")
		}
		parsed, err := url.Parse(s.URL)
		if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
			return fmt.Errorf("http source requires an https URL without credentials")
		}
		if err := validatePayload(s); err != nil {
			return err
		}
	case "file":
		if strings.TrimSpace(s.Path) == "" || s.URL != "" || s.WingetID != "" {
			return fmt.Errorf("file source requires path")
		}
		if err := validatePayload(s); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported source kind")
	}
	return nil
}

func validatePayload(s Source) error {
	switch s.Type {
	case "exe", "msi", "portable":
	case "":
		return fmt.Errorf("source type is required")
	default:
		return fmt.Errorf("unsupported source type")
	}
	integrity := s.Integrity
	if integrity == "" {
		integrity = "sha256"
	}
	switch integrity {
	case "sha256":
		if !validHexHash(s.SHA256) {
			return fmt.Errorf("integrity sha256 requires a valid sha256 digest")
		}
	case "signature":
		if s.SHA256 != "" && !validHexHash(s.SHA256) {
			return fmt.Errorf("invalid sha256 digest")
		}
		if strings.TrimSpace(s.Signer) == "" {
			return fmt.Errorf("signature integrity requires an expected signer")
		}
		if s.Type == "portable" {
			return fmt.Errorf("portable sources cannot rely on signature integrity alone")
		}
	default:
		return fmt.Errorf("unsupported integrity mode")
	}
	if s.Priority < 0 {
		return fmt.Errorf("negative source priority")
	}
	return nil
}

func parseGUID(value string) (string, error) {
	trimmed := strings.TrimSpace(strings.Trim(value, "{}"))
	if len(trimmed) != 36 {
		return "", fmt.Errorf("invalid GUID")
	}
	for i, r := range trimmed {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return "", fmt.Errorf("invalid GUID")
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
				return "", fmt.Errorf("invalid GUID")
			}
		}
	}
	return strings.ToUpper(trimmed), nil
}

func versionLike(value string) bool {
	if strings.TrimSpace(value) == "" || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && r != '.' && r != '-' && r != '_' && r != '+' {
			return false
		}
	}
	return true
}
