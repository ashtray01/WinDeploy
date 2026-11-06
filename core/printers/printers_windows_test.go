//go:build windows

package printers

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWindowsPrintersEnumerate(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	printers, err := (WindowsPrinters{}).Printers(ctx)
	if err != nil {
		t.Fatalf("printer enumeration failed: %v", err)
	}
	defaultName, err := (WindowsDefaults{}).Default(ctx)
	if err != nil {
		t.Fatalf("default printer query failed: %v", err)
	}
	foundDefault := defaultName == ""
	for _, printer := range printers {
		if printer.Name == "" {
			t.Fatal("printer without a name")
		}
		if printer.Status == "" {
			t.Fatalf("printer without status: %+v", printer)
		}
		if defaultName != "" && strings.EqualFold(printer.Name, defaultName) {
			foundDefault = true
			if !printer.Default {
				t.Fatalf("default printer not marked: %+v", printer)
			}
		}
	}
	if !foundDefault {
		t.Fatalf("default printer %q is missing from the enumerated list", defaultName)
	}
	t.Logf("printers=%d default=%q", len(printers), defaultName)
}
