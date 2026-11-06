package profiles

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestYAMLRoundTrip(t *testing.T) {
	p := sampleProfile()
	p.Components[0].Parameters = map[string]string{"credential_ref": "warehouse", "site": "https://example.invalid"}
	data, err := Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	got, err := Load(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, p) {
		t.Fatalf("%+v != %+v", got, p)
	}
	text := "version: 1\nid: office\nname: Офис\ncomponents:\n  - id: network\n    name: Проверка сети\n    category: diagnostics\n    timeout: 30s\n"
	if _, err := Load(strings.NewReader(text)); err != nil {
		t.Fatal(err)
	}
}

func TestYAMLRejectsUnsafeOrInvalidInput(t *testing.T) {
	valid := "version: 1\nid: office\nname: Офис\ncomponents: []\n"
	for _, text := range []string{
		"", "[", "null", valid + "extra: value\n", valid + "id: duplicate\n", valid + "---\n" + valid,
		"version: 1\nid: office\nname: &name Офис\ncomponents: []\n",
		"version: 1\nid: office\nname: *missing\ncomponents: []\n",
		"version: 1\nid: office\nname: Офис\ncomponents: [{id: one, name: One, category: diagnostics, typo: ignored}]\n",
		"version: 1\nid: office\nname: Офис\ncomponents: [{id: one, name: One, category: diagnostics, parameters: {password: do-not-echo}}]\n",
		"version: [do-not-echo]\nid: office\nname: Офис\ncomponents: []\n",
		strings.Repeat(" ", MaxProfileBytes+1),
	} {
		_, err := Load(strings.NewReader(text))
		if err == nil {
			t.Fatalf("invalid YAML accepted: %.100s", text)
		}
		if strings.Contains(err.Error(), "do-not-echo") {
			t.Fatalf("input leaked through error: %v", err)
		}
	}
	p := sampleProfile()
	p.Components[0].Parameters = map[string]string{"secret": "do-not-echo"}
	if _, err := Marshal(p); err == nil {
		t.Fatal("secret exported")
	}
}
