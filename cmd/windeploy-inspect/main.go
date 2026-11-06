// windeploy-inspect exposes read-only inventory and structural profile previews.
package main

import (
	"context"
	"os"
	"os/signal"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	code := run(ctx, os.Args[1:], os.Stdout, os.Stderr, nil)
	cancel()
	os.Exit(code)
}
