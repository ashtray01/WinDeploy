package profiles

import (
	"fmt"
	"reflect"
	"testing"

	"windeploy/core/engine"
)

func TestResolveInheritanceAndIsolation(t *testing.T) {
	base := sampleProfile()
	base.ID = "base"
	base.Components[0].Parameters = map[string]string{"target": "base"}
	base.Components[0].Requirements = engine.OSRequirements{Versions: []string{"windows-10"}, Architectures: []string{"amd64"}}
	base.Components = append(base.Components, Component{ID: "browser", Name: "Browser", Category: "browsers", Parameters: map[string]string{"url": "https://example.invalid"}, Dependencies: []string{"network"}})
	child := Profile{Version: 1, ID: "office", Name: "Офис", Extends: "base", Components: []Component{{ID: "network", Name: "New network", Category: "diagnostics"}, {ID: "extra", Name: "Extra", Category: "diagnostics"}}}
	leaf := Profile{Version: 1, ID: "warehouse", Name: "Склад", Extends: "office", Components: []Component{{ID: "extra", Disabled: true}}}
	registry := map[string]Profile{"base": base, "office": child, "warehouse": leaf}
	resolved, err := Resolve(registry, "warehouse")
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Extends != "" || resolved.ID != "warehouse" || len(resolved.Components) != 3 || resolved.Components[0].Name != "New network" || resolved.Components[0].Parameters != nil || !resolved.Components[2].Disabled {
		t.Fatalf("merge: %+v", resolved)
	}
	resolved.Components[1].Parameters["url"] = "changed"
	resolved.Components[1].Dependencies[0] = "changed"
	if base.Components[1].Parameters["url"] != "https://example.invalid" || base.Components[1].Dependencies[0] != "network" {
		t.Fatal("parent mutated")
	}
	flat, err := Resolve(registry, "base")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(flat, base) {
		t.Fatal("root content changed")
	}
	flat.Components[0].Requirements.Versions[0] = "changed"
	flat.Components[0].Requirements.Architectures[0] = "changed"
	if base.Components[0].Requirements.Versions[0] != "windows-10" || base.Components[0].Requirements.Architectures[0] != "amd64" {
		t.Fatal("requirements aliased")
	}
}

func TestResolveRejectsBrokenRegistry(t *testing.T) {
	a := Profile{Version: 1, ID: "a", Name: "A", Extends: "b"}
	b := Profile{Version: 1, ID: "b", Name: "B", Extends: "a"}
	for _, registry := range []map[string]Profile{{}, {"a": a}, {"a": a, "b": b}, {"a": b}} {
		if _, err := Resolve(registry, "a"); err == nil {
			t.Fatal("invalid registry accepted")
		}
	}
	registry := map[string]Profile{}
	for i := 0; i < 65; i++ {
		id := fmt.Sprintf("p%d", i)
		parent := ""
		if i < 64 {
			parent = fmt.Sprintf("p%d", i+1)
		}
		registry[id] = Profile{Version: 1, ID: id, Name: id, Extends: parent}
	}
	if _, err := Resolve(registry, "p0"); err == nil {
		t.Fatal("unbounded inheritance accepted")
	}
}
