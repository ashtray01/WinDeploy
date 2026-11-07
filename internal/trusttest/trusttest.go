// Package trusttest creates private local state directories for tests that exercise
// the production trust boundary of deployment sessions. It is a test helper only:
// production code must never import it.
package trusttest

import (
	"os"
	"testing"
)

// Dir returns a new temporary directory whose permissions model trusted local
// state: write access only for the current user, SYSTEM and Administrators.
func Dir(t testing.TB) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "windeploy-trusted-*")
	if err != nil {
		t.Fatalf("create trusted test directory: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := restrict(dir); err != nil {
		t.Fatalf("restrict trusted test directory: %v", err)
	}
	return dir
}
