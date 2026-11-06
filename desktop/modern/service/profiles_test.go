package service

import (
	"encoding/json"
	"strings"
	"testing"
	"testing/fstest"
)

func TestProfilesRealEmbeddedPreview(t *testing.T) {
	s := New(&fixtureProvider{})
	defer s.ServiceShutdown()
	catalog, err := s.ListProfiles()
	if err != nil || len(catalog.Profiles) != 8 {
		t.Fatalf("catalog: %v %#v", err, catalog)
	}
	if _, err = s.StartScan(); err != nil {
		t.Fatal(err)
	}
	s.wg.Wait()
	detail, err := s.GetProfile("administrator")
	if err != nil || len(detail.Components) == 0 {
		t.Fatal(err)
	}
	preview, err := s.PreviewProfile("administrator", []string{"extended-network"})
	if err != nil || len(preview.Tasks) != 2 || len(preview.Tasks[1].Task.Dependencies) != 1 {
		t.Fatalf("dependency closure: %#v %v", preview, err)
	}
	legacy, err := s.PreviewProfile("legacy", []string{"network-check"})
	if err != nil || len(legacy.Tasks) != 1 || legacy.Tasks[0].Compatibility != "incompatible" {
		t.Fatalf("legacy: %#v %v", legacy, err)
	}
	empty, err := s.PreviewProfile("base", nil)
	if err != nil || len(empty.Tasks) != 0 {
		t.Fatal("empty selection must stay empty")
	}
	raw, _ := json.Marshal(detail)
	if strings.Contains(string(raw), "parameters") || strings.Contains(string(raw), "Success") {
		t.Fatal("unsafe preview")
	}
	if _, err = s.GetProfile("absent"); err == nil {
		t.Fatal("missing profile accepted")
	}
}
func TestProfilesUnknownTargetAndInvalidYAML(t *testing.T) {
	s := New(&fixtureProvider{})
	defer s.ServiceShutdown()
	if _, err := s.ListProfiles(); err != nil {
		t.Fatal(err)
	}
	p, err := s.PreviewProfile("base", []string{"network-check"})
	if err != nil || len(p.Tasks) == 0 || p.Tasks[0].Compatibility != "unknown" {
		t.Fatalf("unknown target: %v %#v", err, p)
	}
	bad := NewWithProfiles(&fixtureProvider{}, fstest.MapFS{"bad.yaml": {Data: []byte("private: secret")}})
	defer bad.ServiceShutdown()
	catalog, err := bad.ListProfiles()
	if err != nil || len(catalog.Errors) != 1 || len(catalog.Profiles) != 0 || strings.Contains(catalog.Errors[0], "secret") {
		t.Fatalf("invalid catalog: %#v %v", catalog, err)
	}
}

func TestFailedReloadDropsPreviousCatalog(t *testing.T) {
	source := fstest.MapFS{}
	s := NewWithProfiles(&fixtureProvider{}, source)
	defer s.ServiceShutdown()
	// Copy existing embedded YAML through its public filesystem contract.
	embedded := New(&fixtureProvider{})
	defer embedded.ServiceShutdown()
	if _, err := embedded.ListProfiles(); err != nil {
		t.Fatal(err)
	}
	s.registry = embedded.registry
	source["bad.yaml"] = &fstest.MapFile{Data: []byte("invalid: true")}
	if _, err := s.ListProfiles(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.GetProfile("base"); err == nil {
		t.Fatal("old catalog survived reload")
	}
}
