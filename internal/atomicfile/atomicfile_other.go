//go:build !windows

package atomicfile

// POSIX rename replaces the destination atomically and has no sharing conflict
// equivalent, so no error needs a retry.
func isTransient(error) bool { return false }
