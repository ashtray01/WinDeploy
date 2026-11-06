package main

import (
	"strings"
	"testing"

	"windeploy/core/inventory"
)

func TestFormatBytes(t *testing.T) {
	cases := map[uint64]string{0: "—", 512: "512 Б", 1024: "1.0 КиБ", 1536: "1.5 КиБ", 8 << 30: "8.0 ГиБ"}
	for value, want := range cases {
		if got := formatBytes(value); got != want {
			t.Errorf("formatBytes(%d)=%q, want %q", value, got, want)
		}
	}
}

func TestFormatSummary(t *testing.T) {
	text := formatSummary("PC-01", "Windows 7 SP1", "386", "Intel", 2, 2<<30, false, "genuine")
	for _, want := range []string{"PC-01", "Windows 7 SP1", "386", "Intel", "2.0 ГиБ", "нет", "подтверждена"} {
		if !strings.Contains(text, want) {
			t.Errorf("summary missing %q:\n%s", want, text)
		}
	}
	unknown := formatSummary("", "", "", "", 0, 0, true, "unknown")
	if !strings.Contains(unknown, "версия неизвестна") || !strings.Contains(unknown, "есть") || !strings.Contains(unknown, "неизвестно") {
		t.Fatalf("unexpected fallback summary: %s", unknown)
	}
}

func TestShareFromForm(t *testing.T) {
	share, err := shareFromForm(`\\server\share`, "Финансы", `DOMAIN\user`, true)
	if err != nil || share.UNC != `\\server\share` || !share.Shortcut {
		t.Fatalf("valid share rejected: %+v, %v", share, err)
	}
	for _, unc := range []string{"", "server\\share", `\\server`, `\\`, "C:\\share"} {
		if _, err := shareFromForm(unc, "", "", false); err == nil {
			t.Errorf("invalid UNC %q accepted", unc)
		}
	}
}

func TestWindowsFriendly(t *testing.T) {
	cases := map[string]struct {
		version inventory.Version
		want    string
	}{
		"win7":  {inventory.Version{Major: 6, Minor: 1, Build: 7601, ServicePackMajor: 1}, "Windows 7 SP1"},
		"win10": {inventory.Version{Major: 10, Build: 19045}, "Windows 10"},
		"win11": {inventory.Version{Major: 10, Build: 26100}, "Windows 11"},
		"old":   {inventory.Version{Major: 6, Minor: 0, Build: 6002, ServicePackMajor: 2}, "Windows Vista SP2"},
	}
	for name, test := range cases {
		if got := windowsFriendly(test.version); got != test.want {
			t.Errorf("%s: got %q, want %q", name, got, test.want)
		}
	}
}
