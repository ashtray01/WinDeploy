//go:build !smoke

package main

import "testing"

func TestProductionBuildHasNoRemoteDebugging(t *testing.T) {
	if smokeEnabled() {
		t.Fatal("production build enables WebView2 remote debugging")
	}
	if args := smokeBrowserArgs(); len(args) != 0 {
		t.Fatalf("production build exposes debug browser arguments: %v", args)
	}
	if err := validateSmoke(); err != nil {
		t.Fatalf("production build unexpectedly validates a smoke port: %v", err)
	}
}
