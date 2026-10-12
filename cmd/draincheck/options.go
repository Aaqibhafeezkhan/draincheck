package main

import (
	"fmt"
	"slices"
	"strings"
)

// Supported values for --output.
const (
	outputText = "text"
	outputJSON = "json"
)

// wildcard is the --nodes value that selects every node in the cluster.
const wildcard = "*"

// options holds the parsed and validated command-line flags.
type options struct {
	// Nodes lists the nodes to drain. Empty when AllNodes is set.
	Nodes []string
	// AllNodes is set when --nodes is "*"; the nodes are resolved from the
	// cluster inventory later.
	AllNodes   bool
	Kubeconfig string
	Context    string
	Output     string
}

// validate checks the options and normalizes the "*" wildcard into AllNodes.
func (o *options) validate() error {
	if len(o.Nodes) == 0 {
		return fmt.Errorf("--nodes is required (comma-separated node names, or '*' for all nodes)")
	}
	for i, n := range o.Nodes {
		n = strings.TrimSpace(n)
		if n == "" {
			return fmt.Errorf("--nodes: empty node name in list")
		}
		o.Nodes[i] = n
	}
	if slices.Contains(o.Nodes, wildcard) {
		if len(o.Nodes) > 1 {
			return fmt.Errorf("--nodes: '*' cannot be combined with node names")
		}
		o.AllNodes = true
		o.Nodes = nil
	}
	if !slices.Contains([]string{outputText, outputJSON}, o.Output) {
		return fmt.Errorf("invalid --output %q: must be %q or %q", o.Output, outputText, outputJSON)
	}
	return nil
}
