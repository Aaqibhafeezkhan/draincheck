package report

import (
	"bytes"
	"fmt"

	"github.com/Veinar/draincheck/internal/findings"
)

// advice is the text printed once for the findings that share it.
type advice struct {
	why, remediation string
}

// writeCompactAdvice prints the advice of one rule's findings, once per
// distinct advice in order of first appearance; a finding whose case the
// catalog does not hold has none. With one advice it stands alone; with
// several, each names the resources it applies to (the first
// compactLimit of them). A Why, when the case has one, comes right above the
// remediation and shares its resource list. It reports whether a list was
// collapsed.
func writeCompactAdvice(buf *bytes.Buffer, fs []findings.Finding, cat findings.Catalog) bool {
	var order []advice
	resources := map[advice][]string{}
	for _, f := range fs {
		a := advice{why: whyOf(cat, f), remediation: remediationOf(cat, f)}
		if a == (advice{}) {
			continue // no case in the catalog: the finding has no advice
		}
		if _, seen := resources[a]; !seen {
			order = append(order, a)
		}
		resources[a] = append(resources[a], f.Resource.String())
	}

	collapsed := false
	for _, a := range order {
		label := ""
		if len(order) > 1 {
			list, c := joinCollapsed(resources[a])
			collapsed = collapsed || c
			label = " (" + list + ")"
		}
		buf.WriteString("\n")
		switch {
		case a.why == "":
			fmt.Fprintf(buf, "  => Remediation%s: %s\n", label, a.remediation)
		case len(order) > 1:
			fmt.Fprintf(buf, "  => Why%s: %s\n  => Remediation: %s\n", label, a.why, a.remediation)
		default:
			fmt.Fprintf(buf, "  => Why: %s\n  => Remediation: %s\n", a.why, a.remediation)
		}
	}
	return collapsed
}
