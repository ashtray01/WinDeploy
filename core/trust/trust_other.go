//go:build !windows

package trust

// Non-Windows builds have no Windows ACL model. They are not supported targets for
// deployment sessions, so the session store is exercised without an ACL verdict.
func check(string, bool) error { return nil }
