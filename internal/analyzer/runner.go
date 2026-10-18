package analyzer

import (
	"log/slog"

	"github.com/Veinar/draincheck/internal/findings"
)

// Run evaluates every rule against in and returns all findings sorted with
// findings.Sort, so the result does not depend on rule order or on the order
// in which a rule emits its findings. It writes a DEBUG line to logger when
// each rule starts and when it finishes; a nil logger discards them.
func Run(logger *slog.Logger, in Input, rules ...Rule) []findings.Finding {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	var all []findings.Finding
	for _, r := range rules {
		logger.Debug("rule start", "rule", r.ID())
		fs := r.Evaluate(in)
		logger.Debug("rule done", "rule", r.ID(), "findings", len(fs))
		all = append(all, fs...)
	}
	findings.Sort(all)
	return all
}
