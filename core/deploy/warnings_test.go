package deploy

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"windeploy/core/browsers"
	"windeploy/core/engine"
	"windeploy/core/printers"
	"windeploy/core/shares"
)

type warningShortcut struct{ fail bool }

func (s warningShortcut) Create(_ context.Context, spec shares.ShortcutSpec) error {
	if s.fail {
		return errors.New("denied")
	}
	return nil
}
func (s warningShortcut) Matches(context.Context, shares.ShortcutSpec) (bool, error) {
	return !s.fail, nil
}

func TestShareWarningIsPersistedWithOutcome(t *testing.T) {
	connector := &fakeConnector{connections: []shares.Connection{{RemoteName: `\\server\share`}}}
	adapter := &Adapter{
		Shares:           &shares.Manager{Store: fakeShareStore{}, Connector: connector, Shortcuts: warningShortcut{fail: true}},
		ShortcutDir:      `C:\Desktop`,
		CheckUserContext: func(context.Context) error { return nil },
	}
	task := engine.Task{ID: "share", Name: "Share", Category: "shares", Status: engine.Pending, Parameters: map[string]string{"unc": `\\server\share`, "shortcut": "true"}}
	results, err := engine.Run(context.Background(), []engine.Task{task}, adapter)
	if err != nil || len(results) != 1 {
		t.Fatalf("unexpected run: %+v %v", results, err)
	}
	if results[0].Status != engine.Failed {
		t.Fatalf("required shortcut failure reported success: %+v", results[0])
	}
	if len(results[0].Warnings) == 0 || !strings.Contains(strings.Join(results[0].Warnings, " "), "ярлык") {
		t.Fatalf("share warning lost: %q", results[0].Warnings)
	}
	if snapshot, snapErr := engine.NewSnapshot([]engine.Task{task}, results); snapErr != nil || len(snapshot.Results[0].Warnings) == 0 {
		t.Fatalf("warning not persistable: %v", snapErr)
	}
}

type printerProvider struct{ items []printers.Printer }

func testINFPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "printer.inf")
	if err := os.WriteFile(path, []byte("[Version]\nSignature=\"$Windows NT$\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func (p *printerProvider) Printers(context.Context) ([]printers.Printer, error) {
	return append([]printers.Printer{}, p.items...), nil
}

type installingPrinter struct {
	provider *printerProvider
	name     string
}

func (i installingPrinter) AddPrinter(context.Context, printers.AddRequest) error {
	i.provider.items = append(i.provider.items, printers.Printer{Name: i.name, Status: "ok"})
	return nil
}

type failingDefaults struct{}

func (failingDefaults) Default(context.Context) (string, error) { return "", nil }
func (failingDefaults) SetDefault(context.Context, string) error {
	return errors.New("denied")
}

type printerElevation struct{}

func (printerElevation) Elevated(context.Context) (bool, error) { return true, nil }

func TestPrinterDefaultWarningIsPersisted(t *testing.T) {
	provider := &printerProvider{}
	adapter := &Adapter{
		Printers: &printers.Manager{
			// The fake installer makes the queue appear on re-enumeration.
			Printers: provider, Installer: installingPrinter{provider: provider, name: "Office"}, Defaults: failingDefaults{}, Elevation: printerElevation{},
		},
		CheckUserContext: func(context.Context) error { return nil },
	}
	task := engine.Task{ID: "printer", Name: "Printer", Category: "printers", Status: engine.Pending, Parameters: map[string]string{
		"name": "Office", "inf_path": testINFPath(t), "model": "Generic / Text Only", "ip_address": "192.168.1.50", "set_default": "true",
	}}
	results, err := engine.Run(context.Background(), []engine.Task{task}, adapter)
	if err != nil || len(results) != 1 {
		t.Fatalf("unexpected run: %+v %v", results, err)
	}
	if results[0].Status != engine.Success {
		t.Fatalf("printer without confirmed default reported failure: %+v", results[0])
	}
	if len(results[0].Warnings) != 1 || !strings.Contains(results[0].Warnings[0], "по умолчанию") {
		t.Fatalf("printer default warning lost: %q", results[0].Warnings)
	}
}

func TestBrowserWarningIsPersistedOnIncompleteEffects(t *testing.T) {
	adapter := &Adapter{
		Browsers:         &browsers.Manager{Store: noPolicyStore{}},
		CheckUserContext: func(context.Context) error { return nil },
	}
	task := engine.Task{ID: "browser", Name: "Browser", Category: "browsers", Status: engine.Pending, Parameters: map[string]string{"browser": "firefox", "scope": "user", "startup_urls": "https://example.test/"}}
	results, err := engine.Run(context.Background(), []engine.Task{task}, adapter)
	if err != nil || results[0].Status != engine.Failed {
		t.Fatalf("incomplete browser effects reported success: %+v %v", results, err)
	}
	if len(results[0].Warnings) != 1 || !strings.Contains(results[0].Warnings[0], "Firefox") {
		t.Fatalf("browser warning lost: %q", results[0].Warnings)
	}
}
