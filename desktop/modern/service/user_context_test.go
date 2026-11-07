package service

import (
	"context"
	"errors"
	"testing"
	"windeploy/core/browsers"
	"windeploy/core/printers"
	"windeploy/core/settings"
	"windeploy/core/usercontext"
)

func TestUserMutationsRejectUnconfirmedContextBeforeNativeCalls(t *testing.T) {
	s := &Service{userContextCheck: func(context.Context) error { return usercontext.ErrUnconfirmed }}
	for name, call := range map[string]func() error{
		"shares": func() error { _, err := s.AddShare(ShareRequest{}); return err },
		"browser-user": func() error {
			_, err := s.ApplyBrowserPolicy(browsers.Policy{Scope: browsers.ScopeUser}, false)
			return err
		},
		"browser-machine-shortcut": func() error {
			_, err := s.ApplyBrowserPolicy(browsers.Policy{Scope: browsers.ScopeMachine, WebApps: []browsers.WebApp{{Name: "Test"}}}, false)
			return err
		},
		"explorer":            func() error { _, err := s.ApplyExplorer(settings.ExplorerOptions{}, false); return err },
		"default-printer":     func() error { return s.SetDefaultPrinter("test") },
		"printer-add-default": func() error { _, err := s.AddPrinter(printers.AddRequest{SetDefault: true}); return err },
		"test-page":           func() error { return s.PrintTestPage("test") },
		"packages":            func() error { _, err := s.StartPackageInstall([]string{"test"}); return err },
	} {
		t.Run(name, func(t *testing.T) {
			if err := call(); !errors.Is(err, usercontext.ErrUnconfirmed) {
				t.Fatalf("native boundary reached: %v", err)
			}
		})
	}
}
