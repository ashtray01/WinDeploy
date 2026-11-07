//go:build windows

package usercontext

import (
	"context"
	"testing"
)

func TestNativeIdentityReadOnly(t *testing.T) {
	identity, err := (Native{}).Read(context.Background())
	if err != nil {
		if (Native{}).Check(context.Background()) == nil {
			t.Fatal("unknown native context allowed")
		}
		t.Log("native shell identity unavailable; gate denies")
		return
	}
	if identity.ProcessSID == "" || identity.ShellSID == "" {
		t.Fatal("empty native identity")
	}
	if identity.ProcessSID == identity.ShellSID {
		t.Log("native process and shell SIDs match")
	} else {
		t.Log("native process and shell SIDs differ; user mutations blocked")
	}
}
