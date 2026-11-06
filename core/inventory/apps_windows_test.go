//go:build windows

package inventory

import (
	"context"
	"testing"
)

func TestNativeAppsReadOnly(t *testing.T) {
	apps, err := (Native{}).Apps(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, app := range apps {
		if app.Name == "" || len(app.Views) == 0 || (app.Scope != "machine" && app.Scope != "current-user") {
			t.Fatal("invalid app metadata")
		}
	}
}
