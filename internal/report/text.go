package report

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"time"

	"github.com/Veinar/draincheck/internal/findings"
)

// textSeverities is the order sections appear in: most severe first.
var textSeverities = []findings.Severity{findings.Blocked, findings.Risk, findings.Unknown}

// WriteText writes fs to w as human-readable text, grouped by severity with
// BLOCKED first. taken is the time of the snapshot the findings describe. It is
// shown in the header and in the closing disclaimer. Findings are sorted on a
// copy, so the same input always renders the same bytes. An empty input is
// reported as "No findings reported."; the output never states that a drain is
// safe.
//
// By default the layout is compact: findings are grouped by rule inside each
// severity section and each remediation is printed once. With verbose set,
// every finding is printed in full with its own remediation. flags adds a
// section between the summary and the findings when a drain flag is missing or
// eviction is disabled; with neither, the report is unchanged.
func WriteText(w io.Writer, fs []findings.Finding, taken time.Time, verbose bool, flags DrainFlags) error {
	sorted := slices.Clone(fs)
	findings.Sort(sorted)

	var buf bytes.Buffer
	buf.WriteString("draincheck - preflight maintenance analysis\n")
	fmt.Fprintf(&buf, "Timestamp: %s\n\n", formatTimestamp(taken))
	writeSummary(&buf, sorted)
	writeDrainFlags(&buf, flags)

	if len(sorted) == 0 {
		buf.WriteString("\nNo findings reported.\n")
	}
	collapsed := false
	for _, sev := range textSeverities {
		if verbose {
			writeVerboseSection(&buf, sev, sorted)
			continue
		}
		if writeCompactSection(&buf, sev, sorted) {
			collapsed = true
		}
	}
	if collapsed {
		buf.WriteString("\nSome lists are collapsed. Use --verbose to print every finding.\n")
	}

	fmt.Fprintf(&buf, "\n%s\n", Disclaimer(taken))

	_, err := w.Write(buf.Bytes())
	return err
}

func writeSummary(buf *bytes.Buffer, fs []findings.Finding) {
	buf.WriteString("=== FINDINGS SUMMARY ===\n")
	for _, sev := range textSeverities {
		// "RISK:" is padded so the counts line up under "BLOCKED:" and "UNKNOWN:".
		fmt.Fprintf(buf, "  %-8s %d\n", string(sev)+":", countSeverity(fs, sev))
	}
}

func countSeverity(fs []findings.Finding, sev findings.Severity) int {
	n := 0
	for _, f := range fs {
		if f.Severity == sev {
			n++
		}
	}
	return n
}
