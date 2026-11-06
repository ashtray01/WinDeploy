package profiles

import (
	"strings"
	"testing"

	"windeploy/core/engine"
)

func sampleProfile() Profile {
	return Profile{Version: 1, ID: "office", Name: "Офис", Components: []Component{{ID: "network", Name: "Сеть", Category: "diagnostics", Timeout: "30s"}}}
}

func TestProfileValidation(t *testing.T) {
	if err := sampleProfile().Validate(); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		edit func(*Profile)
	}{
		{"version", func(p *Profile) { p.Version = 2 }},
		{"ID", func(p *Profile) { p.ID = "../escape" }},
		{"name", func(p *Profile) { p.Name = " " }},
		{"self parent", func(p *Profile) { p.Extends = p.ID }},
		{"duplicate", func(p *Profile) { p.Components = append(p.Components, p.Components[0]) }},
		{"category", func(p *Profile) { p.Components[0].Category = "arbitrary-command" }},
		{"timeout", func(p *Profile) { p.Components[0].Timeout = "-1s" }},
		{"invalid timeout", func(p *Profile) { p.Components[0].Timeout = "tomorrow" }},
		{"architecture", func(p *Profile) {
			p.Components[0].Requirements = engine.OSRequirements{Architectures: []string{"arm64"}}
		}},
		{"disabled config", func(p *Profile) { p.Components[0].Disabled = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			p := sampleProfile()
			test.edit(&p)
			if err := p.Validate(); err == nil {
				t.Fatal("invalid profile accepted")
			}
		})
	}
}

func TestProfileSecretPolicy(t *testing.T) {
	for _, params := range []map[string]string{{"password": "do-not-echo"}, {"API-Key": "do-not-echo"}, {"source": "https://user:do-not-echo@example.invalid/a"}, {"access_token": "do-not-echo"}} {
		p := sampleProfile()
		p.Components[0].Parameters = params
		err := p.Validate()
		if err == nil || strings.Contains(err.Error(), "do-not-echo") {
			t.Fatalf("unsafe validation: %v", err)
		}
	}
	p := sampleProfile()
	p.Components[0].Parameters = map[string]string{"credential_ref": "warehouse-share"}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	p.Components = append(p.Components, Component{ID: "inherited", Disabled: true})
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
}
