package trusttest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"windeploy/core/trust"
)

func TestDirIsTrusted(t *testing.T) {
	dir := Dir(t)
	if err := trust.Check(context.Background(), dir, true); err != nil {
		t.Fatalf("trusted helper directory rejected: %v", err)
	}
	file := filepath.Join(dir, "state.json")
	if err := os.WriteFile(file, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := trust.Check(context.Background(), file, false); err != nil {
		t.Fatalf("file in trusted helper directory rejected: %v", err)
	}
}
