package report

import (
	"bytes"
	"fmt"

	"github.com/Veinar/draincheck/internal/findings"
)

// writeVerboseSection writes the findings of one severity in full: the
// description and the remediation are repeated for every finding.
func writeVerboseSection(buf *bytes.Buffer, sev findings.Severity, fs []findings.Finding) {
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
