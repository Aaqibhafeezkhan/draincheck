package analyzer

import (
	"maps"

	"github.com/Veinar/draincheck/internal/findings"
)

// DefaultCatalog returns the cases of every default rule. The report prints
// their text, and a finding carries only its own facts and the name of its
// case. The map is a copy: the caller may keep or change it.
func DefaultCatalog() findings.Catalog {
	cat := findings.Catalog{}
	for _, part := range []findings.Catalog{
		pdbDisruptionBlockedCases,
		pdbOperatorManagedCases(),
		pdbUnsatisfiableCases,
		pdbOverlappingSelectorsCases,
		pdbBatchExceedsBudgetCases,
		pdbStaleStatusCases,
		pdbSyncFailedCases,
		nodeNotSchedulableCases,
		noRemainingNodesCases,
		workloadUntoleratedTaintsCases,
		singleReplicaWorkloadCases,
		allReplicasOnTargetsCases,
		podOwnershipCases,
		podLocalStorageCases,
		missingResourceRequestsCases,
		podStuckTerminatingCases,
	} {
		maps.Copy(cat, part)
	}
	return cat
}
