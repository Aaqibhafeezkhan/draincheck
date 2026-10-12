package main

import "github.com/spf13/cobra"

func newRootCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:           "draincheck",
		Short:         "Read-only preflight analysis for multi-node kubectl drain",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
}
