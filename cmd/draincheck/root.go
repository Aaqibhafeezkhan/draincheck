package main

import (
	"context"

	"github.com/spf13/cobra"
)

// newRootCmd builds the root command. run receives the validated options.
func newRootCmd(version string, run func(context.Context, options) error) *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:           "draincheck",
		Short:         "Read-only preflight analysis for multi-node kubectl drain",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(c *cobra.Command, _ []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			return run(c.Context(), opts)
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&opts.Nodes, "nodes", nil, "comma-separated names of the nodes to drain, or '*' for all nodes")
	f.StringVar(&opts.Kubeconfig, "kubeconfig", "", "path to the kubeconfig file (default: standard loading rules)")
	f.StringVar(&opts.Context, "context", "", "kubeconfig context to use (default: current context)")
	f.StringVar(&opts.Output, "output", outputText, "output format: text or json")
	f.BoolVar(&opts.Verbose, "verbose", false, "text output: print every finding in full instead of the compact layout")
	f.StringVar(&opts.FailOn, "fail-on", failOnBlocked, "exit 1 when a finding is at or above this level: blocked, risk or any")
	f.BoolVar(&opts.Debug, "debug", false, "write diagnostic lines to stderr; the report on stdout does not change")
	f.BoolVar(&opts.IgnoreDaemonSets, "ignore-daemonsets", false, "as kubectl drain: ignore DaemonSet-managed pods")
	f.BoolVar(&opts.DeleteEmptyDirData, "delete-emptydir-data", false, "as kubectl drain: continue even if there are pods using emptyDir")
	f.BoolVar(&opts.Force, "force", false, "as kubectl drain: continue even if there are pods that do not declare a controller")
	f.StringVar(&opts.PodSelector, "pod-selector", "", "as kubectl drain: label selector to filter the pods on the nodes")
	f.BoolVar(&opts.DisableEviction, "disable-eviction", false, "as kubectl drain: delete pods instead of evicting them, which bypasses PodDisruptionBudgets")
	return cmd
}
