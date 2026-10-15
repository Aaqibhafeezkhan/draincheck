package report

import (
	"bytes"
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
)

// compactLimit is how many findings, or resources in a remediation, are listed
// before the rest is collapsed into "... and M more".
const compactLimit = 5

// writeCompactSection writes the findings of one severity grouped by rule: a
// header with the count, one line per finding, then each distinct remediation
// once. It reports whether any list was collapsed.
func writeCompactSection(buf *bytes.Buffer, sev findings.Severity, fs []findings.Finding) bool {
	n := countSeverity(fs, sev)
	if n == 0 {
		return false
	}
	fmt.Fprintf(buf, "\n=== %s (%d) ===\n", sev, n)

	byRule := map[string][]findings.Finding{}
	for _, f := range fs {
		if f.Severity == sev {
			byRule[f.RuleID] = append(byRule[f.RuleID], f)
		}
	}
	rules := make([]string, 0, len(byRule))
	for id := range byRule {
		rules = append(rules, id)
	}
	slices.Sort(rules)

	collapsed := false
	for _, id := range rules {
		if writeCompactRule(buf, id, byRule[id]) {
			collapsed = true
		}
	}
	return collapsed
}

func writeCompactRule(buf *bytes.Buffer, id string, fs []findings.Finding) bool {
	noun := "findings"
	if len(fs) == 1 {
		noun = "finding"
	}
	fmt.Fprintf(buf, "\n[%s] %d %s\n", id, len(fs), noun)

	collapsed := false
	for i, f := range fs {
		if i == compactLimit {
			fmt.Fprintf(buf, "  ... and %d more\n", len(fs)-compactLimit)
			collapsed = true
			break
		}
		fmt.Fprintf(buf, "  %s: %s\n", f.Resource, f.Description)
	}

	// Distinct remediations in order of first appearance.
	var texts []string
	resources := map[string][]string{}
	for _, f := range fs {
		if _, seen := resources[f.Remediation]; !seen {
			texts = append(texts, f.Remediation)
		}
		resources[f.Remediation] = append(resources[f.Remediation], f.Resource.String())
	}
	if len(texts) == 1 {
		fmt.Fprintf(buf, "  Remediation: %s\n", texts[0])
		return collapsed
	}
	for _, text := range texts {
		list, c := joinCollapsed(resources[text])
		collapsed = collapsed || c
		fmt.Fprintf(buf, "  Remediation (%s): %s\n", list, text)
	}
	return collapsed
}

// joinCollapsed joins names with ", ", keeping the first compactLimit and
// ending with "and M more" when the list is longer. It reports whether it
// collapsed.
func joinCollapsed(names []string) (string, bool) {
	if len(names) <= compactLimit {
		return strings.Join(names, ", "), false
	}
	return fmt.Sprintf("%s and %d more", strings.Join(names[:compactLimit], ", "), len(names)-compactLimit), true
}
