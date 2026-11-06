package catalog

import (
	"strings"
	"testing"
)

func TestEmbeddedCatalogLoads(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatalf("embedded catalog is invalid: %v", err)
	}
	if catalog.Version != 1 {
		t.Fatalf("unexpected catalog version: %d", catalog.Version)
	}
	if len(catalog.Packages) != 12 {
		t.Fatalf("expected 12 packages, got %d", len(catalog.Packages))
	}
	required := []string{
		"sevenzip", "google-chrome", "mozilla-firefox", "yandex-browser",
		"telegram-desktop", "tg-ws-proxy", "max-messenger", "vlc",
		"mpc-hc", "sumatrapdf", "irfanview", "printerone",
	}
	for _, id := range required {
		pkg, ok := catalog.Find(id)
		if !ok {
			t.Fatalf("package %q is missing", id)
		}
		if len(pkg.Detections) == 0 || len(pkg.Sources) == 0 {
			t.Fatalf("package %q has no detection or sources", id)
		}
		for _, source := range pkg.Sources {
			if source.Kind == "http" {
				if !strings.HasPrefix(source.URL, "https://") {
					t.Fatalf("package %q has a non-TLS URL", id)
				}
				if source.SHA256 == "" {
					t.Fatalf("package %q has an unpinned HTTP source", id)
				}
			}
		}
	}
}

func TestCatalogSourcePriorities(t *testing.T) {
	catalog, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range catalog.Packages {
		for _, source := range pkg.Sources {
			if source.Priority < 0 {
				t.Fatalf("package %q has a negative priority", pkg.ID)
			}
		}
	}
}
