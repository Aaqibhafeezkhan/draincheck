package report

import (
	"bytes"
	"fmt"

	"github.com/Veinar/draincheck/internal/findings"
)

// writeVerify prints the commands that confirm f, one per line under a Verify
// label, each indented one level deeper than the label at indent. It prints
// nothing for a finding with no commands.
func writeVerify(buf *bytes.Buffer, f findings.Finding, indent string) {
	if len(f.Verify) == 0 {
		return
	}
	fmt.Fprintf(buf, "%sVerify:\n", indent)
	for _, c := range f.Verify {
		fmt.Fprintf(buf, "%s  %s\n", indent, c)
	}
}

// showsVerifyCompact reports whether the compact layout prints the commands of
// a finding of this severity: the ones that stop or cannot be judged, where the
// reader has to check at once. A RISK keeps the list short; --verbose has them.
func showsVerifyCompact(sev findings.Severity) bool {
	return sev == findings.Blocked || sev == findings.Unknown
}
