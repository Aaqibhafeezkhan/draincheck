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
func WriteText(w io.Writer, fs []findings.Finding, taken time.Time) error {
	sorted := slices.Clone(fs)
	findings.Sort(sorted)

	var buf bytes.Buffer
	buf.WriteString("draincheck - preflight maintenance analysis\n")
	fmt.Fprintf(&buf, "Timestamp: %s\n\n", formatTimestamp(taken))
	writeSummary(&buf, sorted)

	if len(sorted) == 0 {
		buf.WriteString("\nNo findings reported.\n")
	}
	for _, sev := range textSeverities {
		writeSection(&buf, sev, sorted)
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

func writeSection(buf *bytes.Buffer, sev findings.Severity, fs []findings.Finding) {
	n := countSeverity(fs, sev)
	if n == 0 {
		return
	}
	fmt.Fprintf(buf, "\n=== %s (%d) ===\n", sev, n)
	for _, f := range fs {
		if f.Severity != sev {
			continue
		}
		fmt.Fprintf(buf, "\n[%s] %s\n", f.RuleID, f.Resource)
		fmt.Fprintf(buf, "  %s\n", f.Description)
		fmt.Fprintf(buf, "  Remediation: %s\n", f.Remediation)
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
