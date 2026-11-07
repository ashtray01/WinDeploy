package deploy

import (
	"context"
	"testing"
	"windeploy/core/browsers"
	"windeploy/core/engine"
	"windeploy/core/shares"
)

func TestExistingShareRequiresCorrectShortcutAndRepairsWithoutReconnect(t *testing.T) {
	a, connector := shareAdapter()
	connector.connections = []shares.Connection{{RemoteName: `\\server\share`}}
	task := engine.Task{ID: "share", Name: "Share", Category: "shares", Status: engine.Pending, Parameters: map[string]string{"unc": `\\server\share`, "shortcut": "true"}}
	if status, err := a.Precheck(context.Background(), task); err != nil || status != engine.Pending {
		t.Fatalf("missing shortcut skipped: %s %v", status, err)
	}
	if _, err := a.Execute(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if connector.requests != 0 {
		t.Fatal("repair reconnected existing UNC")
	}
	if err := a.Verify(context.Background(), task); err != nil {
		t.Fatal(err)
	}
	if status, err := a.Precheck(context.Background(), task); err != nil || status != engine.AlreadyInstalled {
		t.Fatalf("verified repeat not idempotent: %s %v", status, err)
	}
	links := a.Shares.Shortcuts.(*fakeShortcut)
	links.specs[0].Target = `\\wrong\share`
	if status, err := a.Precheck(context.Background(), task); err != nil || status != engine.Pending {
		t.Fatalf("wrong shortcut target skipped: %s %v", status, err)
	}
	if err := a.Verify(context.Background(), task); err == nil {
		t.Fatal("wrong shortcut target verified")
	}
}

type noPolicyStore struct{}

func (noPolicyStore) WriteString(context.Context, browsers.Scope, browsers.Browser, string, string) error {
	return nil
}
func (noPolicyStore) WriteDWord(context.Context, browsers.Scope, browsers.Browser, string, uint32) error {
	return nil
}
func (noPolicyStore) DeleteValue(context.Context, browsers.Scope, browsers.Browser, string) error {
	return nil
}

func TestIncompleteBrowserEffectsBlockDependency(t *testing.T) {
	a := &Adapter{CheckUserContext: func(context.Context) error { return nil }, Browsers: &browsers.Manager{Store: noPolicyStore{}}}
	tasks := []engine.Task{{ID: "browser", Name: "Browser", Category: "browsers", Status: engine.Pending, Parameters: map[string]string{"browser": "firefox", "scope": "user", "startup_urls": "https://example.test/"}}, {ID: "after", Name: "After", Category: "diagnostics", Status: engine.Pending, Dependencies: []string{"browser"}}}
	results, err := engine.Run(context.Background(), tasks, a)
	if err != nil || results[0].Status != engine.Failed || results[1].Status != engine.Skipped {
		t.Fatalf("incomplete effects reported success: %+v %v", results, err)
	}
}
