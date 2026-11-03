package report

import (
	"fmt"

	"github.com/Veinar/draincheck/internal/findings"
)

// compactSummary returns the totals in the header of a collapsed rule:
// how many findings it has and, when any has a namespace, in how many
// namespaces. Cluster-scoped findings are not counted as a namespace. It is a
// pure function of fs, so the order of fs does not matter.
func compactSummary(fs []findings.Finding) string {
	namespaces := map[string]struct{}{}
	for _, f := range fs {
		if f.Resource.Namespace != "" {
			namespaces[f.Resource.Namespace] = struct{}{}
		}
	}
	switch len(namespaces) {
	case 0:
		return fmt.Sprintf("%d findings", len(fs))
	case 1:
		return fmt.Sprintf("%d findings in 1 namespace", len(fs))
	default:
		return fmt.Sprintf("%d findings in %d namespaces", len(fs), len(namespaces))
	}
}
