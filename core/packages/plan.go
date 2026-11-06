package packages

import (
	"context"
	"fmt"
	"time"

	"windeploy/core/engine"
	"windeploy/core/inventory"
)

// WingetProbe detects the package manager without installing or upgrading
// anything. Version is informational and may be empty.
type WingetProbe interface {
	Available(context.Context) (bool, string)
}

// Plan is a read-only preview. It never downloads or installs.
type Plan struct {
	GeneratedAt time.Time            `json:"generated_at"`
	System      inventory.SystemInfo `json:"system"`
	Items       []PlanItem           `json:"items"`
	Warnings    []string             `json:"warnings"`
}

// PlanItem is one package decision. Source fields are empty when no source is
// available. ReasonCode is stable for UI translation; Reason is an English
// diagnostic that is safe to log.
type PlanItem struct {
	PackageID     string                `json:"package_id"`
	Name          string                `json:"name"`
	Publisher     string                `json:"publisher"`
	Category      string                `json:"category"`
	Group         string                `json:"group,omitempty"`
	Description   string                `json:"description,omitempty"`
	Requirements  engine.OSRequirements `json:"requirements"`
	Action        Action                `json:"action"`
	ReasonCode    string                `json:"reason_code"`
	Reason        string                `json:"reason"`
	Installed     InstalledState        `json:"installed"`
	SourceID      string                `json:"source_id,omitempty"`
	SourceKind    string                `json:"source_kind,omitempty"`
	SourceType    string                `json:"source_type,omitempty"`
	SourceVersion string                `json:"source_version,omitempty"`
	Integrity     string                `json:"integrity,omitempty"`
	Signer        string                `json:"signer,omitempty"`
	SizeBytes     int64                 `json:"size_bytes,omitempty"`
	RequiresAdmin bool                  `json:"requires_admin"`
}

// Installable reports whether the plan asks for a real install/update.
func (i PlanItem) Installable() bool { return i.Action == ActionInstall || i.Action == ActionUpdate }

// Planner builds plans from catalog, facts and system capabilities.
type Planner struct {
	Catalog Catalog
	Facts   Facts
	System  inventory.SystemInfo
	Winget  WingetProbe
}

// Build validates the selection and creates one item per selected package in
// catalog order. Empty selection means every catalog package.
func (p Planner) Build(ctx context.Context, selection []string) (Plan, error) {
	if err := p.Catalog.Validate(); err != nil {
		return Plan{}, err
	}
	if p.Facts == nil {
		return Plan{}, fmt.Errorf("facts provider is required")
	}
	if err := p.Catalog.ValidateSelection(selection); err != nil {
		return Plan{}, err
	}
	chosen := selection
	if len(chosen) == 0 {
		chosen = make([]string, 0, len(p.Catalog.Packages))
		for _, pkg := range p.Catalog.Packages {
			chosen = append(chosen, pkg.ID)
		}
	}
	selected := map[string]bool{}
	for _, id := range chosen {
		selected[id] = true
	}
	wingetAvailable := false
	if p.Winget != nil {
		wingetAvailable, _ = p.Winget.Available(ctx)
	}
	plan := Plan{GeneratedAt: time.Now().UTC(), System: p.System, Items: []PlanItem{}, Warnings: []string{}}
	osKnown := p.System.Windows.Token() != "" && (p.System.Architecture == "386" || p.System.Architecture == "amd64")
	if !osKnown {
		plan.Warnings = append(plan.Warnings, "target OS or architecture is unknown; compatibility cannot be confirmed")
	}
	for _, pkg := range p.Catalog.Packages {
		if !selected[pkg.ID] {
			continue
		}
		if err := ctx.Err(); err != nil {
			return Plan{}, err
		}
		item, err := p.itemFor(ctx, pkg, wingetAvailable, osKnown)
		if err != nil {
			return Plan{}, err
		}
		plan.Items = append(plan.Items, item)
	}
	return plan, nil
}

func (p Planner) itemFor(ctx context.Context, pkg Package, wingetAvailable, osKnown bool) (PlanItem, error) {
	item := PlanItem{
		PackageID: pkg.ID, Name: pkg.Name, Publisher: pkg.Publisher, Category: pkg.Category,
		Group: pkg.Group, Description: pkg.Description, Requirements: pkg.Requirements,
		RequiresAdmin: pkg.RequiresAdmin, Action: ActionUnknown,
	}
	applicable := false
	osErr := error(nil)
	if osKnown {
		applicable, osErr = inventory.Applicable(pkg.Requirements, p.System)
	}
	source, sourceReason, hasSource := chooseSource(ctx, p.Facts, pkg, p.System, wingetAvailable)
	state, detectErr := Detect(ctx, p.Facts, pkg)
	item.Installed = state

	switch {
	case osKnown && osErr == nil && !applicable:
		item.Action, item.ReasonCode = ActionIncompatible, "os-incompatible"
		item.Reason = "OS or architecture requirements are not met"
		return item, nil
	case osKnown && osErr != nil:
		item.Action, item.ReasonCode = ActionUnknown, "os-unknown"
		item.Reason = "OS requirements could not be evaluated"
		return item, nil
	case !hasSource:
		item.Action, item.ReasonCode = ActionUnavailable, sourceReason
		item.Reason = "no applicable installation source"
		return item, nil
	}

	item.SourceID, item.SourceKind, item.SourceType = source.ID, source.Kind, source.Type
	if item.SourceType == "" {
		item.SourceType = source.Kind
	}
	item.SourceVersion, item.Signer, item.SizeBytes = source.Version, source.Signer, source.SizeBytes
	item.Integrity = source.Integrity
	if item.Integrity == "" {
		item.Integrity = "sha256"
	}
	if detectErr != nil && !state.Installed {
		item.Action, item.ReasonCode = ActionUnknown, "detection-unavailable"
		item.Reason = "installed software could not be enumerated"
		return item, nil
	}
	if state.Installed {
		if source.Version == "" {
			item.Action, item.ReasonCode = ActionUpToDate, "installed"
			item.Reason = "installed version detected; source version is not pinned"
			return item, nil
		}
		if VersionAtLeast(state.Version, source.Version) {
			item.Action, item.ReasonCode = ActionUpToDate, "up-to-date"
			item.Reason = "installed version is current"
			return item, nil
		}
		item.Action, item.ReasonCode = ActionUpdate, "update-available"
		item.Reason = "catalog source is newer than the installed version"
		return item, nil
	}
	item.Action, item.ReasonCode = ActionInstall, "not-installed"
	item.Reason = "package is not installed"
	return item, nil
}

// chooseSource applies architecture, availability and priority rules. A source
// with an unknown requirement is never selected silently.
func chooseSource(ctx context.Context, facts Facts, pkg Package, system inventory.SystemInfo, wingetAvailable bool) (Source, string, bool) {
	order := make([]int, 0, len(pkg.Sources))
	for i := range pkg.Sources {
		order = append(order, i)
	}
	for i := 1; i < len(order); i++ {
		for j := i; j > 0; j-- {
			left, right := pkg.Sources[order[j-1]], pkg.Sources[order[j]]
			if left.Priority < right.Priority || (left.Priority == right.Priority && order[j-1] < order[j]) {
				break
			}
			order[j-1], order[j] = order[j], order[j-1]
		}
	}
	reason := "no-source"
	for _, index := range order {
		source := pkg.Sources[index]
		if source.Arch != "" && source.Arch != system.Architecture {
			continue
		}
		switch source.Kind {
		case "winget":
			if wingetAvailable {
				return source, "", true
			}
			reason = "winget-unavailable"
		case "file":
			info, err := facts.File(ctx, source.Path)
			if err != nil {
				continue
			}
			if info.Exists {
				return source, "", true
			}
			reason = "file-source-missing"
		case "http":
			return source, "", true
		}
	}
	return Source{}, reason, false
}
