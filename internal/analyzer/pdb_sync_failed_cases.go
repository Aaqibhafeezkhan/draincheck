package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of PDBSyncFailed. A finding names one; the text is below.
const (
	pdbSyncFailedSyncFailed = "sync_failed"
)

// pdbSyncFailedCases holds the text of each case, keyed as findings.CaseKey does.
var pdbSyncFailedCases = findings.Catalog{
	findings.CaseKey(PDBSyncFailedID, pdbSyncFailedSyncFailed): {
		Why:         "The numbers in its status cannot be trusted, so draincheck cannot tell whether eviction of its pods will be refused.",
		Remediation: "Confirm with `kubectl get pdb -n <namespace> <name> -o jsonpath='{.status.conditions}'` (reason SyncFailed and its message). Then fix what the disruption controller reports, usually a pod owner that has no scale subresource (look at it with `kubectl get <kind> -n <namespace> <owner> -o yaml`) or a selector it cannot resolve, and run draincheck again. Until then the budget cannot be relied on either way.",
	},
}
