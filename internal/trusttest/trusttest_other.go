//go:build !windows

package trusttest

// Other platforms have no Windows ACL model; the session trust check is a no-op
// there, so the temporary directory needs no extra restriction.
func restrict(string) error { return nil }
