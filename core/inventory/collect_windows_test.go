//go:build windows

package inventory

import (
	"context"
	"testing"
)

func TestNativeInventoryIntegrationReadOnly(t *testing.T) {
	r, err := Collect(context.Background(), Native{})
	if err != nil {
		t.Fatalf("native inventory failed: %v; sections: %+v", err, r.Sections)
	}
	if !r.Complete || r.System == nil || r.CPU == nil || r.Memory == nil || len(r.Sections) != 7 {
		t.Fatal("incomplete native inventory")
	}
}
