package findings

import (
	"cmp"
	"slices"
)

// Sort orders fs in place by resource namespace, kind and name, so output is
// deterministic. RuleID and Severity break ties between findings that share a
// resource.
func Sort(fs []Finding) {
	slices.SortStableFunc(fs, func(a, b Finding) int {
		return cmp.Or(
			cmp.Compare(a.Resource.Namespace, b.Resource.Namespace),
			cmp.Compare(a.Resource.Kind, b.Resource.Kind),
			cmp.Compare(a.Resource.Name, b.Resource.Name),
			cmp.Compare(a.RuleID, b.RuleID),
			cmp.Compare(a.Severity, b.Severity),
		)
	})
}
