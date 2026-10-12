package main

import "github.com/spf13/cobra"

// newRootCmd builds the root command. run receives the validated options.
func newRootCmd(version string, run func(options) error) *cobra.Command {
	var opts options
	cmd := &cobra.Command{
		Use:           "draincheck",
		Short:         "Read-only preflight analysis for multi-node kubectl drain",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(*cobra.Command, []string) error {
			if err := opts.validate(); err != nil {
				return err
			}
			return run(opts)
		},
	}
	f := cmd.Flags()
	f.StringSliceVar(&opts.Nodes, "nodes", nil, "comma-separated names of the nodes to drain, or '*' for all nodes")
	f.StringVar(&opts.Kubeconfig, "kubeconfig", "", "path to the kubeconfig file (default: standard loading rules)")
	f.StringVar(&opts.Context, "context", "", "kubeconfig context to use (default: current context)")
	f.StringVar(&opts.Output, "output", outputText, "output format: text or json")
	return cmd
}
