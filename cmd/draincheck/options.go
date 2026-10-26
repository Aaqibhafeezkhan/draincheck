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
	// Verbose prints every finding in full in the text report.
	Verbose bool
	// FailOn is the lowest finding level that makes the run exit 1.
	FailOn string
	// Debug writes diagnostic lines to stderr.
	Debug bool
	// The kubectl drain flags the operator will pass; the analysis judges the
	// pods as kubectl drain with the same flags would.
	IgnoreDaemonSets   bool
	DeleteEmptyDirData bool
	Force              bool
	DisableEviction    bool
	// PodSelector is a label selector; "" selects every pod.
	PodSelector string
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
	if _, err := o.drainOptions(); err != nil {
		return err
	}
	return validFailOn(o.FailOn)
}
