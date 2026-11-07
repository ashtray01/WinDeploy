//go:build smoke

package main

import (
	"flag"
	"fmt"
	"strconv"
)

// This file is compiled only into local diagnostic/smoke builds. The release
// build must not expose a remote WebView2 debugging port.
var smokePort = flag.Int("smoke-port", 0, "opt-in localhost WebView2 debugging for UI smoke tests")

func smokeEnabled() bool { return *smokePort != 0 }

func validateSmoke() error {
	if *smokePort != 0 && (*smokePort < 1024 || *smokePort > 65535) {
		return fmt.Errorf("smoke port out of range")
	}
	return nil
}

func smokeBrowserArgs() []string {
	if !smokeEnabled() {
		return nil
	}
	return []string{"--remote-debugging-address=127.0.0.1", "--remote-debugging-port=" + strconv.Itoa(*smokePort)}
}
