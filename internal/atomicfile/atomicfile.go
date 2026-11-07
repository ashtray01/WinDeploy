// Package atomicfile replaces files through a temporary sibling so readers never
// observe a partially written document. On Windows the replacement can fail with a
// transient access error while antivirus or an indexer still holds the previous
// version, so Replace retries that specific conflict a bounded number of times and
// returns the original error when the storage stays busy.
package atomicfile

import (
	"context"
	"os"
	"time"
)

const (
	replaceAttempts   = 6
	replaceBaseDelay  = 10 * time.Millisecond
	replaceMaxDelay   = 160 * time.Millisecond
	replaceTotalLimit = 2 * time.Second
)

// Replace atomically renames temp to target, replacing an existing target.
// Only transient Windows sharing conflicts are retried; any other error is
// returned immediately. Exhausted retries return the last error, never success.
func Replace(ctx context.Context, temp, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	started := time.Now()
	delay := replaceBaseDelay
	var lastErr error
	for attempt := 1; attempt <= replaceAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		lastErr = os.Rename(temp, target)
		if lastErr == nil {
			return nil
		}
		if !isTransient(lastErr) || attempt == replaceAttempts || time.Since(started)+delay > replaceTotalLimit {
			return lastErr
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}
		if delay *= 2; delay > replaceMaxDelay {
			delay = replaceMaxDelay
		}
	}
	return lastErr
}
