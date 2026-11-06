package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of PDBBatchExceedsBudget. A finding names one; the text is below.
const (
	pdbBatchExceedsBudgetBatchExceeds = "batch_exceeds"
)

// pdbBatchExceedsBudgetCases holds the text of each case, keyed as findings.CaseKey does.
var pdbBatchExceedsBudgetCases = findings.Catalog{
	findings.CaseKey(PDBBatchExceedsBudgetID, pdbBatchExceedsBudgetBatchExceeds): {
		Why:         "The drain waits for replacements to become Ready between evictions, so it takes longer; nothing is lost.",
		Remediation: "Confirm with `kubectl get pdb -n <namespace> <name>` (ALLOWED DISRUPTIONS) and compare it with the pods named in the finding. Then, safest first: expect the drain to take longer, nothing is lost; to drain faster, add replicas or relax minAvailable/maxUnavailable on the PodDisruptionBudget, or drain the nodes one after another.",
	},
}
