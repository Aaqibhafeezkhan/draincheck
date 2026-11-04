package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of PDBOverlappingSelectors. A finding names one; the text is below.
const (
	pdbOverlappingSelectorsOverlapping = "overlapping"
)

// pdbOverlappingSelectorsCases holds the text of each case, keyed as findings.CaseKey does.
var pdbOverlappingSelectorsCases = findings.Catalog{
	findings.CaseKey(PDBOverlappingSelectorsID, pdbOverlappingSelectorsOverlapping): {
		Why:         "The eviction API refuses a pod covered by more than one PodDisruptionBudget, and kubectl drain reports that error at once and does not retry it.",
		Remediation: "Confirm with `kubectl get pod -n <namespace> <pod> --show-labels` and `kubectl describe pdb -n <namespace> <pdb>` for each budget named in the finding (the selector of each). Then narrow the PodDisruptionBudget selectors so that each pod is covered by exactly one budget, or merge the budgets, before draining. Waiting does not help: the eviction API refuses a pod covered by several budgets whatever they allow. Run draincheck again afterwards: the budgets are checked on their own once they no longer overlap.",
	},
}
