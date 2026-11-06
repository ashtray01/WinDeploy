package inventory

import "testing"

func TestVersionClassification(t *testing.T) {
	for _, test := range []struct {
		v    Version
		want string
	}{
		{Version{Major: 6, Minor: 1, Build: 7601, ServicePackMajor: 1, ProductType: 1}, "windows-7-sp1"},
		{Version{Major: 6, Minor: 1, Build: 7600, ProductType: 1}, ""},
		{Version{Major: 10, Build: 19045, ProductType: 1}, "windows-10"},
		{Version{Major: 10, Build: 22000, ProductType: 1}, "windows-11"},
		{Version{Major: 10, Build: 26100, ProductType: 3}, ""},
		{Version{Major: 11, Build: 10000, ProductType: 1}, ""},
		{Version{}, ""},
	} {
		if got := test.v.Token(); got != test.want {
			t.Errorf("%+v: %q != %q", test.v, got, test.want)
		}
	}
	for code, want := range map[uint16]string{0: "386", 9: "amd64", 12: "arm64", 99: "unknown"} {
		if architecture(code) != want {
			t.Fatal("architecture mismatch")
		}
	}
}
