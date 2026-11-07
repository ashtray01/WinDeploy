package buildinfo

import "testing"

func TestDisplayVersionFallsBackToDevelopmentIdentity(t *testing.T) {
	oldVersion, oldCommit := Version, Commit
	t.Cleanup(func() { Version, Commit = oldVersion, oldCommit })

	Version, Commit = "", ""
	if got := DisplayVersion(); got != "0.0.0-dev" {
		t.Fatalf("unexpected fallback version %q", got)
	}
	if got := DisplayCommit(); got != "dev" {
		t.Fatalf("unexpected fallback commit %q", got)
	}
}

func TestDisplayCommitIsBounded(t *testing.T) {
	oldCommit := Commit
	t.Cleanup(func() { Commit = oldCommit })
	Commit = "0123456789abcdef0123456789abcdef"
	if got := DisplayCommit(); len(got) > 12 {
		t.Fatalf("commit label is too long: %q", got)
	}
}
