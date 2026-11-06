package service

import (
	"strings"
	"testing"
)

func TestParseNetworkTarget(t *testing.T) {
	cases := []struct {
		raw   string
		kind  string
		host  string
		port  int
		fails bool
	}{
		{"dns:example.com", "dns", "example.com", 0, false},
		{"tcp:fileserver:445", "tcp", "fileserver", 445, false},
		{"tcp:[::1]:445", "tcp", "::1", 445, false},
		{"icmp:8.8.8.8", "icmp", "8.8.8.8", 0, false},
		{"smb:fileserver", "smb", "fileserver", 0, false},
		{"http://example.test/", "", "", 0, true},
		{"tcp:fileserver", "", "", 0, true},
		{"tcp:fileserver:0", "", "", 0, true},
		{"unknown:value", "", "", 0, true},
		{"dns:", "", "", 0, true},
	}
	for _, test := range cases {
		target, err := parseNetworkTarget(test.raw)
		if test.fails {
			if err == nil {
				t.Errorf("%q unexpectedly accepted", test.raw)
			}
			continue
		}
		if err != nil {
			t.Errorf("%q rejected: %v", test.raw, err)
			continue
		}
		if string(target.kind) != test.kind || target.host != test.host || target.port != test.port {
			t.Errorf("%q parsed as %+v", test.raw, target)
		}
	}
}

func TestAddShareRejectsInvalidUNC(t *testing.T) {
	service := &Service{}
	_, err := service.AddShare(ShareRequest{UNC: `C:\local\path`})
	if err == nil || !strings.Contains(err.Error(), "UNC") {
		t.Fatalf("invalid UNC accepted: %v", err)
	}
}

func TestAddShareDryRunDoesNotTouchSystem(t *testing.T) {
	service := &Service{}
	view, err := service.AddShare(ShareRequest{UNC: `\\fileserver\finance`, Name: "Finance", Shortcut: true, DryRun: true})
	if err != nil {
		t.Fatalf("dry-run failed: %v", err)
	}
	if view.Connected || view.CredentialSaved {
		t.Fatalf("dry-run changed state: %+v", view)
	}
	if !strings.Contains(view.Message, "Пробный режим") {
		t.Fatalf("unexpected dry-run message: %s", view.Message)
	}
}

func TestListShareConnectionsIsReadOnly(t *testing.T) {
	service := &Service{}
	connections, err := service.ListShareConnections()
	if err != nil {
		t.Fatalf("enumeration failed: %v", err)
	}
	for _, connection := range connections {
		if connection.RemoteName == "" {
			t.Fatalf("connection without a remote name: %+v", connection)
		}
	}
}
