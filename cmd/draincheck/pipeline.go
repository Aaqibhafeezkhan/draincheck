package main

import (
	"context"
	"fmt"
	"io"

	"github.com/Veinar/draincheck/internal/analyzer"
	"github.com/Veinar/draincheck/internal/inventory"
	"github.com/Veinar/draincheck/internal/report"
)

// execute collects a snapshot, runs the default rules against the target
// nodes and writes the report to w. It returns an *exitError with
// exitFindings when the findings reach opts.FailOn.
func execute(ctx context.Context, opts options, c inventory.Collector, w io.Writer) error {
	if err := checkOutputImplemented(opts); err != nil {
		return err
	}

	snap, err := c.Collect(ctx)
	if err != nil {
		return fmt.Errorf("collect cluster state: %w", err)
	}
	targets, err := inventory.ResolveTargetNodes(snap, opts.Nodes, opts.AllNodes)
	if err != nil {
		return err
	}

	fs := analyzer.Run(analyzer.Input{Snapshot: snap, Targets: targets}, analyzer.DefaultRules()...)
	if err := report.WriteJSON(w, fs); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	if shouldFail(fs, opts.FailOn) {
		return &exitError{code: exitFindings}
	}
	return nil
}

// checkOutputImplemented rejects output formats that are accepted by the flag
// but have no formatter yet. run calls it before connecting to the cluster.
func checkOutputImplemented(opts options) error {
	if opts.Output != outputJSON {
		return &exitError{code: exitFailure, msg: fmt.Sprintf("--output %s is not implemented yet; use --output json", opts.Output)}
	}
	return nil
}
