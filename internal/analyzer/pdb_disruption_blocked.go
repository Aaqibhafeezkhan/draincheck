package analyzer

import (
	"fmt"

	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
)

// PDBDisruptionBlockedID is the rule ID reported by PDBDisruptionBlocked.
const PDBDisruptionBlockedID = "PDB_DISRUPTION_BLOCKED"

// PDBDisruptionBlocked reports PodDisruptionBudgets whose status allows zero
// disruptions while they select pods on target nodes, so the eviction API will
// refuse to evict those pods.
//
// One BLOCKED finding is emitted per budget, pointing at the budget, however
// many pods it selects: ten replicas behind one budget are one finding. The
// selected pods and their nodes are in the typed evidence and none is dropped;
// a pod selected by two budgets is listed under each. Pods that kubectl drain
// never evicts are skipped (DaemonSet pods and static pods, see
// skippedByDrain), as are pods in a terminal phase (Succeeded, Failed),
// because eviction does not consult budgets for them. A budget left with no
// pod gives no finding. A budget with an invalid selector is skipped: it
// cannot be matched reliably.
//
// A budget whose status is stale (see PDBStaleStatus) is skipped too: its
// numbers cannot be trusted, so PDBStaleStatus reports it as UNKNOWN instead.
//
// A budget that selects a pod controlled by a custom resource (an operator, see
// operatorOwned) is not BLOCKED but UNKNOWN: whether the drain stops depends on
// operator state draincheck cannot see. Those budgets are grouped per operator
// and namespace, see operatorManagedFindings. Budgets whose pods are controlled
// by Deployments, StatefulSets, Jobs and the like stay BLOCKED.
type PDBDisruptionBlocked struct{}

// ID implements Rule.
func (PDBDisruptionBlocked) ID() string { return PDBDisruptionBlockedID }

// Evaluate implements Rule.
func (PDBDisruptionBlocked) Evaluate(in Input) []findings.Finding {
	var out []findings.Finding
	var operatorManaged []pdbSelection
	for _, sel := range selectedPDBs(in) {
		pdb := sel.pdb
		if pdb.Status.DisruptionsAllowed != 0 || pdbStale(pdb) {
			continue
		}
		if len(sel.operators) > 0 {
			operatorManaged = append(operatorManaged, sel)
			continue
		}
		out = append(out, findings.Finding{
			RuleID:   PDBDisruptionBlockedID,
			Severity: findings.Blocked,
			Resource: sel.resource(),
			Evidence: sel.evidence(),
			Description: fmt.Sprintf(
				"PodDisruptionBudget %s/%s allows 0 disruptions (%d healthy, %d desired), so eviction of its %s on %s will be refused.",
				pdb.Namespace, pdb.Name, pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy,
				plural(len(sel.pods), "pod"), plural(len(sel.nodes), "target node")),
			Remediation: "Add healthy replicas, or relax minAvailable/maxUnavailable on the PodDisruptionBudget, before draining.",
		})
	}
	return append(out, operatorManagedFindings(operatorManaged)...)
}

func terminal(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
}
