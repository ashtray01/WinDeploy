package inventory

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestAppsViewsDeduplicationAndPartialErrors(t *testing.T) {
	calls := 0
	apps, err := collectApps(context.Background(), "amd64", func(_ context.Context, scope, view string) ([]appEntry, error) {
		calls++
		if scope == "machine" && view == "64" {
			return []appEntry{{Key: "native", Name: "Same", Version: "2"}}, errors.New("private diagnostic")
		}
		return []appEntry{{Key: "shared", Name: " Same ", Version: "1"}, {Key: "hidden", Name: "Hidden", Hidden: true}, {Key: "unnamed"}}, nil
	})
	if err == nil || strings.Contains(err.Error(), "private") || calls != 4 || len(apps) != 3 {
		t.Fatalf("%+v %v calls=%d", apps, err, calls)
	}
	found := false
	for _, app := range apps {
		if app.Scope == "current-user" && app.Key == "shared" {
			found = true
			if len(app.Views) != 2 {
				t.Fatal("shared views duplicated")
			}
		}
	}
	if !found {
		t.Fatal("current-user apps missing")
	}
	calls = 0
	if _, err := collectApps(context.Background(), "386", func(context.Context, string, string) ([]appEntry, error) { calls++; return nil, nil }); err != nil || calls != 2 {
		t.Fatalf("32-bit views: %d %v", calls, err)
	}
	if _, err := collectApps(context.Background(), "unknown", nil); err == nil {
		t.Fatal("unknown architecture accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := collectApps(ctx, "amd64", func(context.Context, string, string) ([]appEntry, error) {
		t.Fatal("registry read after cancel")
		return nil, nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
