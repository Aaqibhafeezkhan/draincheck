package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/Veinar/draincheck/internal/analyzer"
	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	"github.com/Veinar/draincheck/internal/report"
)

// execute collects a snapshot, runs the default rules against the target
// nodes and writes the report to w. It returns an *exitError with
// exitFindings when the findings reach opts.FailOn. Each step writes a DEBUG
// line to logger, which never touches w; a nil logger discards them.
func execute(ctx context.Context, opts options, logger *slog.Logger, c inventory.Collector, w io.Writer) error {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	snap, err := c.Collect(ctx)
	if err != nil {
		return fmt.Errorf("collect cluster state: %w", err)
	}
	targets, err := inventory.ResolveTargetNodes(snap, opts.Nodes, opts.AllNodes)
	if err != nil {
		logger.Debug("resolve target nodes failed", "requested", opts.Nodes, "all", opts.AllNodes, "error", err)
		return err
	}
	logger.Debug("resolve target nodes", "requested", opts.Nodes, "all", opts.AllNodes, "targets", targets)

	drain, err := opts.drainOptions()
	if err != nil {
		logger.Debug("drain options failed", "error", err)
		return err
	}
	in := analyzer.Input{Snapshot: snap, Targets: targets, Drain: drain}
	if opts.PodSelector != "" {
		selected, leftOut := analyzer.CountSelectedPods(in)
		logger.Debug("pod selector", "selector", opts.PodSelector, "selected", selected, "left_out", leftOut)
	}

	fs := analyzer.Run(logger, in, analyzer.DefaultRules()...)
	logSummary(logger, fs, opts.FailOn)

	logger.Debug("write report", "format", opts.Output, "verbose", opts.Verbose, "findings", len(fs))
	if err := writeReport(w, opts.Output, opts.Verbose, fs, snap.Timestamp()); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	if shouldFail(fs, opts.FailOn) {
		return &exitError{code: exitFindings}
	}
	return nil
}

// logSummary writes the number of findings per severity and the --fail-on
// level they are held against.
func logSummary(logger *slog.Logger, fs []findings.Finding, failOn string) {
	counts := map[findings.Severity]int{}
	for _, f := range fs {
		counts[f.Severity]++
	}
	logger.Debug("summary",
		"blocked", counts[findings.Blocked], "risk", counts[findings.Risk], "unknown", counts[findings.Unknown],
		"fail_on", failOn)
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
