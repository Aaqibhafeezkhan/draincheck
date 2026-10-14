// Command draincheck analyzes a Kubernetes cluster and reports what would
// block or put at risk a planned multi-node drain. It never modifies the cluster.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/Veinar/draincheck/internal/inventory"
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

// run connects to the cluster and executes the analysis.
func run(ctx context.Context, opts options) error {
	if err := checkOutputImplemented(opts); err != nil {
		return err
	}
	client, err := newClientset(opts.Kubeconfig, opts.Context)
	if err != nil {
		return err
	}
	return execute(ctx, opts, inventory.NewClientsetCollector(client), os.Stdout)
}
