//go:build !smoke

package main

// Production builds deliberately do not register --smoke-port and never enable
// WebView2 remote debugging or DevTools. The same UI is smoke-tested with the
// private `smoke` build tag; that artifact is never packaged for release.

func smokeEnabled() bool { return false }

func validateSmoke() error { return nil }

func smokeBrowserArgs() []string { return nil }
