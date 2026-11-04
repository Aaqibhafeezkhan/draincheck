package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of PDBStaleStatus. A finding names one; the text is below.
const (
	pdbStaleStatusStaleStatus = "stale_status"
)

// pdbStaleStatusCases holds the text of each case, keyed as findings.CaseKey does.
var pdbStaleStatusCases = findings.Catalog{
	findings.CaseKey(PDBStaleStatusID, pdbStaleStatusStaleStatus): {
		Why:         "The numbers in its status are out of date, so draincheck cannot tell whether eviction of its pods will be refused.",
		Remediation: "Confirm with `kubectl get pdb -n <namespace> <name> -o jsonpath='{.metadata.generation} {.status.observedGeneration}'` (two different numbers: the status is out of date). Then wait for the disruption controller to update the PodDisruptionBudget status and run draincheck again. If it stays out of date, check that kube-controller-manager is healthy (on a kubeadm cluster `kubectl get pods -n kube-system -l component=kube-controller-manager`).",
	},
}
