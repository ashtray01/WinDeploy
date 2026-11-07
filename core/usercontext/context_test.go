package usercontext

import (
	"context"
	"errors"
	"testing"
)

func TestIdentityGate(t *testing.T) {
	for _, test := range []struct {
		name     string
		identity Identity
		err      error
		allow    bool
	}{
		{"same", Identity{"S-1-5-21-1", "S-1-5-21-1"}, nil, true},
		{"other-admin", Identity{"S-1-5-21-2", "S-1-5-21-1"}, nil, false},
		{"unknown-shell", Identity{ProcessSID: "S-1-5-21-1"}, nil, false},
		{"unknown-process", Identity{ShellSID: "S-1-5-21-1"}, nil, false},
		{"read-error", Identity{}, errors.New("denied"), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := Check(context.Background(), func(context.Context) (Identity, error) { return test.identity, test.err })
			if (err == nil) != test.allow {
				t.Fatalf("allow=%v err=%v", test.allow, err)
			}
		})
	}
	if err := Check(context.Background(), nil); !errors.Is(err, ErrUnconfirmed) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := Check(ctx, func(context.Context) (Identity, error) { t.Fatal("cancelled reader invoked"); return Identity{}, nil }); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
