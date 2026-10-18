// Command draincheck analyzes a Kubernetes cluster and reports what would
// block or put at risk a planned multi-node drain. It never modifies the cluster.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := newRootCmd(version, run).ExecuteContext(ctx); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintln(os.Stderr, "error:", msg)
		}
		stop()
		os.Exit(exitCodeOf(err))
	}
}
