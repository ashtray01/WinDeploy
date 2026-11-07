//go:build windows

package shares

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeShortcutReadbackTargetAndArguments(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "local-target.txt")
	if err := os.WriteFile(target, []byte("not executed"), 0o600); err != nil {
		t.Fatal(err)
	}
	spec := ShortcutSpec{Path: filepath.Join(dir, "Проверка.lnk"), Target: target, Arguments: "--app=https://example.test/"}
	adapter := WindowsShortcut{}
	ctx := context.Background()
	if err := adapter.Create(ctx, spec); err != nil {
		t.Fatal(err)
	}
	if matches, err := adapter.Matches(ctx, spec); err != nil || !matches {
		t.Fatalf("native saved link mismatch: %v %v", matches, err)
	}
	wrong := spec
	wrong.Arguments = "--app=https://other.test/"
	if matches, err := adapter.Matches(ctx, wrong); err != nil || matches {
		t.Fatalf("wrong arguments accepted: %v %v", matches, err)
	}
	wrong = spec
	wrong.Target = filepath.Join(dir, "other.txt")
	if matches, err := adapter.Matches(ctx, wrong); err != nil || matches {
		t.Fatalf("wrong target accepted: %v %v", matches, err)
	}
	if err := os.Remove(spec.Path); err != nil {
		t.Fatal(err)
	}
	if matches, err := adapter.Matches(ctx, spec); err != nil || matches {
		t.Fatalf("missing link accepted: %v %v", matches, err)
	}
	if err := os.WriteFile(spec.Path, []byte("corrupt"), 0o600); err != nil {
		t.Fatal(err)
	}
	if matches, err := adapter.Matches(ctx, spec); err == nil || matches {
		t.Fatalf("corrupt link accepted: %v %v", matches, err)
	}
}
