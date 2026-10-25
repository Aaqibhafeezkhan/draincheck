// Command draincheck analyzes a Kubernetes cluster and reports what would
// block or put at risk a planned multi-node drain. It never modifies the cluster.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ignoreClosedPipes()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := newRootCmd(version, run).ExecuteContext(ctx); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, "error:", msg)
		}
		stop()
		os.Exit(exitCodeOf(err))
	}
}
