package main

import (
	"testing"

	"github.com/wailsapp/wails/v3/pkg/application"
	"windeploy/core/inventory"
)

func TestCoreAndDesktopContractsRemainSeparate(t *testing.T) {
	options := application.Options{Name: "WinDeploy"}
	if options.Name == "" || (inventory.Version{Major: 10, Build: 26100, ProductType: 1}).Token() != "windows-11" {
		t.Fatal("desktop/core contract unavailable")
	}
}
