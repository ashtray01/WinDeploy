// Package profiles validates declarative workstation configuration. It never
// runs system operations; adapters interpret its task parameters separately.
package profiles

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"windeploy/core/engine"
)

const SchemaVersion = 1

var identifier = regexp.MustCompile(`^[a-z][a-z0-9._-]{0,63}$`)

type Profile struct {
	Version    int         `yaml:"version" json:"version"`
	ID         string      `yaml:"id" json:"id"`
	Name       string      `yaml:"name" json:"name"`
	Extends    string      `yaml:"extends,omitempty" json:"extends,omitempty"`
	Components []Component `yaml:"components" json:"components"`
}

// Component carries non-secret adapter configuration only. Use credential_ref
// for an external credential identifier, never actual passwords/tokens. Validation
// rejects common secret keys and URL userinfo but cannot recognize every secret.
// Disabled is a tombstone containing only ID and Disabled, for inheritance.
type Component struct {
	ID           string                `yaml:"id" json:"id"`
	Name         string                `yaml:"name,omitempty" json:"name,omitempty"`
	Category     string                `yaml:"category,omitempty" json:"category,omitempty"`
	Parameters   map[string]string     `yaml:"parameters,omitempty" json:"parameters,omitempty"`
	Requirements engine.OSRequirements `yaml:"requirements,omitempty" json:"requirements,omitempty"`
	Dependencies []string              `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`
	Timeout      string                `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	ResourceKey  string                `yaml:"resource_key,omitempty" json:"resource_key,omitempty"`
	Disabled     bool                  `yaml:"disabled,omitempty" json:"disabled,omitempty"`
}

// Validate checks this profile, allowing unresolved inherited dependencies.
// Full dependency existence/cycle validation happens after inheritance resolution.
func (p Profile) Validate() error {
	if p.Version != SchemaVersion {
		return fmt.Errorf("unsupported profile schema version")
	}
	if !identifier.MatchString(p.ID) || strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("profile ID and name are required")
	}
	if p.Extends != "" && (!identifier.MatchString(p.Extends) || p.Extends == p.ID) {
		return fmt.Errorf("invalid parent profile ID")
	}
	seen := map[string]bool{}
	for i, c := range p.Components {
		if !identifier.MatchString(c.ID) || seen[c.ID] {
			return fmt.Errorf("invalid or duplicate component ID at index %d", i)
		}
		seen[c.ID] = true
		if err := c.validate(); err != nil {
			return fmt.Errorf("component at index %d: %w", i, err)
		}
	}
	return nil
}

func (c Component) validate() error {
	if c.Disabled {
		if c.Name != "" || c.Category != "" || len(c.Parameters) > 0 || len(c.Dependencies) > 0 || len(c.Requirements.Versions) > 0 || len(c.Requirements.Architectures) > 0 || c.Timeout != "" || c.ResourceKey != "" {
			return fmt.Errorf("disabled component must contain only id and disabled")
		}
		return nil
	}
	switch c.Category {
	case "packages", "drivers", "shares", "printers", "browsers", "diagnostics", "windows":
	default:
		return fmt.Errorf("unsupported component category")
	}
	for key, value := range c.Parameters {
		normalized := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(key))
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("empty parameter key")
		}
		if normalized != "credentialref" && (strings.Contains(normalized, "password") || strings.Contains(normalized, "passwd") || strings.Contains(normalized, "token") || strings.Contains(normalized, "secret") || strings.Contains(normalized, "privatekey") || normalized == "credentials" || normalized == "authorization" || normalized == "apikey") {
			return fmt.Errorf("secret parameter forbidden; use credential_ref")
		}
		if parsed, err := url.Parse(value); err == nil && parsed.User != nil {
			return fmt.Errorf("URL credentials are forbidden")
		}
	}
	for _, arch := range c.Requirements.Architectures {
		if arch != "386" && arch != "amd64" {
			return fmt.Errorf("unsupported architecture")
		}
	}
	for _, version := range c.Requirements.Versions {
		switch version {
		case "windows-7-sp1", "windows-10", "windows-11":
		default:
			return fmt.Errorf("unsupported Windows version")
		}
	}
	task, err := c.task()
	if err != nil {
		return err
	}
	return task.Validate()
}

func (c Component) task() (engine.Task, error) {
	var timeout time.Duration
	var err error
	if c.Timeout != "" {
		timeout, err = time.ParseDuration(c.Timeout)
		if err != nil {
			return engine.Task{}, fmt.Errorf("invalid component timeout")
		}
	}
	return engine.Task{ID: c.ID, Name: c.Name, Category: c.Category, Parameters: c.Parameters, Requirements: c.Requirements, Dependencies: c.Dependencies, Timeout: timeout, Status: engine.Pending, ResourceKey: c.ResourceKey}, nil
}
