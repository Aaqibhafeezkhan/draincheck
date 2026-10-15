package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Veinar/draincheck/internal/analyzer"
	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	"github.com/Veinar/draincheck/internal/report"
)

// execute collects a snapshot, runs the default rules against the target
// nodes and writes the report to w. It returns an *exitError with
// exitFindings when the findings reach opts.FailOn.
func execute(ctx context.Context, opts options, c inventory.Collector, w io.Writer) error {
	snap, err := c.Collect(ctx)
	if err != nil {
		return fmt.Errorf("collect cluster state: %w", err)
	}
	targets, err := inventory.ResolveTargetNodes(snap, opts.Nodes, opts.AllNodes)
	if err != nil {
		return err
	}

	fs := analyzer.Run(analyzer.Input{Snapshot: snap, Targets: targets}, analyzer.DefaultRules()...)
	if err := writeReport(w, opts.Output, opts.Verbose, fs, snap.Timestamp()); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	if shouldFail(fs, opts.FailOn) {
		return &exitError{code: exitFindings}
	}
	return nil
}

// writeReport renders fs with the formatter chosen by --output; verbose only
// affects the text formatter, JSON always carries every finding. options
// validation guarantees format is one of the known values.
func writeReport(w io.Writer, format string, verbose bool, fs []findings.Finding, taken time.Time) error {
	if format == outputJSON {
		return report.WriteJSON(w, fs, taken)
	}
	return report.WriteText(w, fs, taken, verbose)
}
