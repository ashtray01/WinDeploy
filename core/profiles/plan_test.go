package profiles

import (
	"os"
	"path/filepath"
	"testing"

	"windeploy/core/engine"
)

func TestBuildSelectedPlanAndClone(t *testing.T) {
	p := sampleProfile()
	p.Components[0].Parameters = map[string]string{"kind": "network"}
	p.Components = append(p.Components, Component{ID: "app", Name: "App", Category: "packages", Dependencies: []string{"network"}, ResourceKey: engine.WindowsInstallerResource}, Component{ID: "unused", Name: "Unused", Category: "diagnostics"})
	tasks, err := BuildPlan(p, []string{"app"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 2 || tasks[0].ID != "network" || tasks[1].ID != "app" || tasks[1].Status != engine.Pending || tasks[1].ResourceKey != engine.WindowsInstallerResource {
		t.Fatalf("%+v", tasks)
	}
	tasks[0].Parameters["kind"] = "changed"
	tasks[1].Dependencies[0] = "changed"
	if p.Components[0].Parameters["kind"] != "network" || p.Components[1].Dependencies[0] != "network" {
		t.Fatal("plan aliases profile")
	}
	if empty, err := BuildPlan(p, []string{}); err != nil || len(empty) != 0 {
		t.Fatalf("empty selection: %+v %v", empty, err)
	}
	if all, err := BuildPlan(p, nil); err != nil || len(all) != 3 {
		t.Fatalf("all selection: %+v %v", all, err)
	}
	clone, err := Clone(p, "custom", "Мой профиль")
	if err != nil {
		t.Fatal(err)
	}
	clone.Components[0].Parameters["kind"] = "changed"
	if p.ID != "office" || p.Components[0].Parameters["kind"] != "network" {
		t.Fatal("clone aliases source")
	}
	preview, err := Preview(p, []string{"app"})
	if err != nil || len(preview.Tasks) != 2 {
		t.Fatalf("preview: %+v %v", preview, err)
	}
}

func TestPlanRejectsBrokenSelection(t *testing.T) {
	p := sampleProfile()
	for _, selection := range [][]string{{"missing"}, {"network", "network"}} {
		if _, err := BuildPlan(p, selection); err == nil {
			t.Fatal("invalid selection accepted")
		}
	}
	p.Extends = "base"
	if _, err := BuildPlan(p, nil); err == nil {
		t.Fatal("unresolved profile accepted")
	}
	p.Extends = ""
	p.Components[0].Dependencies = []string{"missing"}
	if _, err := BuildPlan(p, nil); err == nil {
		t.Fatal("missing dependency accepted")
	}
	p.Components = append(p.Components, Component{ID: "missing", Disabled: true})
	if _, err := BuildPlan(p, nil); err == nil {
		t.Fatal("disabled dependency accepted")
	}
	p.Components[1] = Component{ID: "missing", Name: "M", Category: "diagnostics", Dependencies: []string{"network"}}
	if _, err := BuildPlan(p, nil); err == nil {
		t.Fatal("dependency cycle accepted")
	}
}

func TestExampleProfilesProduceValidPlans(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("..", "..", "profiles", "*.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 8 {
		t.Fatalf("expected 8 examples, found %d", len(paths))
	}
	registry := map[string]Profile{}
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		p, err := Load(file)
		file.Close()
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if _, exists := registry[p.ID]; exists {
			t.Fatal("duplicate profile")
		}
		registry[p.ID] = p
	}
	for id := range registry {
		t.Run(id, func(t *testing.T) {
			p, err := Resolve(registry, id)
			if err != nil {
				t.Fatal(err)
			}
			tasks, err := BuildPlan(p, nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(tasks) == 0 {
				t.Fatal("empty example plan")
			}
			if _, err = engine.PreviewPlan(tasks); err != nil {
				t.Fatal(err)
			}
		})
	}
}
