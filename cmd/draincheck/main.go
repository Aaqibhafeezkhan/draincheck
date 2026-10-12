// Command draincheck analyzes a Kubernetes cluster and reports what would
// block or put at risk a planned multi-node drain. It never modifies the cluster.
package main

import (
	"fmt"
	"os"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	if err := newRootCmd(version).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(2)
	}
}
