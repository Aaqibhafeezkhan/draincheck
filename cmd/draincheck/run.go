package main

import (
	"context"
	"io"
	"log/slog"
	"os"

	"github.com/Veinar/draincheck/internal/inventory"
)

// run connects to the cluster and executes the analysis.
func run(ctx context.Context, opts options) error {
	return runWith(ctx, opts, os.Stdout, os.Stderr)
}

// runWith is run with its output streams given: the report goes to stdout and,
// with --debug, the diagnostic lines go to stderr.
func runWith(ctx context.Context, opts options, stdout, stderr io.Writer) error {
	logger := newLogger(opts.Debug, stderr)
	err := analyze(ctx, opts, logger, stdout)
	logger.Debug("finished", "exit_code", exitCode(err))
	return err
}

// analyze connects to the cluster and executes the analysis, logging to logger.
func analyze(ctx context.Context, opts options, logger *slog.Logger, stdout io.Writer) error {
	logSettings(logger, opts)
	client, err := newClientset(opts.Kubeconfig, opts.Context)
	if err != nil {
		return err
	}
	return execute(ctx, opts, logger, inventory.NewClientsetCollector(client, logger), stdout)
}

// logSettings writes the settings the run uses. It names kubeconfig files and
// never reads or prints what is in them.
func logSettings(logger *slog.Logger, opts options) {
	logger.Debug("settings",
		"kubeconfig", kubeconfigPaths(opts.Kubeconfig), "context", opts.Context,
		"nodes", opts.Nodes, "all_nodes", opts.AllNodes,
		"output", opts.Output, "verbose", opts.Verbose, "fail_on", opts.FailOn)
}
