package analyzer

import (
	"fmt"

	"github.com/Veinar/draincheck/internal/findings"
)

// PDBBatchExceedsBudgetID is the rule ID reported by PDBBatchExceedsBudget.
const PDBBatchExceedsBudgetID = "PDB_BATCH_EXCEEDS_BUDGET"

// PDBBatchExceedsBudget reports PodDisruptionBudgets that allow some
// disruptions, but fewer than the whole batch of target nodes would cause.
// Pods from several target nodes covered by the same budget share it, so the
// pods are counted per budget over all target nodes and compared with
// status.disruptionsAllowed. This is the set-aware check kubectl drain cannot
// make: it drains one node at a time and sees each node fit.
//
// The severity is RISK, not BLOCKED. With at least one disruption allowed the
// first eviction succeeds, its replacement restores the budget once it is
// Ready, and the drain proceeds in steps: it waits, it does not stall. A budget
// already at 0 is the stall case and stays with PDBDisruptionBlocked, so one
// budget is never reported by both.
//
// One finding is emitted per budget, pointing at the budget, with the pods
// and nodes in typed evidence. The pods counted are the ones the other PDB
// rules count (see selectedPDBs). A stale budget (see PDBStaleStatus), one the
// controller failed to compute (see PDBSyncFailed) and one that selects a pod
// controlled by an operator (see operatorOwned) are skipped: their numbers say
// nothing reliable about the drain. So is a budget that selects a pod another
// budget selects too (see overlappedPDBs): the controller chooses one of
// them arbitrarily, and PDBOverlappingSelectors reports the pod.
//
// Only pods that use the budget up are counted: a pod that is not Ready and
// that the eviction API lets past the budget (see evictedPastBudget) is not,
// and neither are pods that bypass every budget (see podDrain.bypassesBudget). A budget
// whose unhealthyPodEvictionPolicy is a value draincheck does not know is
// skipped, since it cannot be counted reliably.
type PDBBatchExceedsBudget struct{}

// ID implements Rule.
func (PDBBatchExceedsBudget) ID() string { return PDBBatchExceedsBudgetID }

// Evaluate implements Rule.
func (PDBBatchExceedsBudget) Evaluate(in Input) []findings.Finding {
	var out []findings.Finding
	for _, sel := range selectedPDBs(in) {
		pdb := sel.pdb
		allowed := pdb.Status.DisruptionsAllowed
		if sel.overlapped || allowed <= 0 || len(sel.pods) <= int(allowed) || pdbStale(pdb) || !sel.policyKnown || len(sel.operators) > 0 {
			continue
		}
		if _, failed := pdbSyncFailure(pdb); failed {
			continue
		}
		out = append(out, findings.Finding{
			RuleID:   PDBBatchExceedsBudgetID,
			Severity: findings.Risk,
			Resource: sel.resource(),
			Evidence: findings.PDBBatchEvidence{
				PDBEvidence: sel.evidence(),
				PodsToEvict: len(sel.pods),
			},
			Description: fmt.Sprintf(
				"Draining %s evicts %s selected by PodDisruptionBudget %s/%s, which allows %s at once, so the drain will wait for replacements to become Ready between evictions.",
				plural(len(sel.nodes), "target node"), plural(len(sel.pods), "pod"),
				pdb.Namespace, pdb.Name, plural(int(allowed), "disruption")),
			Remediation: "Confirm with `kubectl get pdb -n <namespace> <name>` (ALLOWED DISRUPTIONS) and compare it with the pods named in the finding. Then, safest first: expect the drain to take longer, nothing is lost; to drain faster, add replicas or relax minAvailable/maxUnavailable on the PodDisruptionBudget, or drain the nodes one after another.",
		})
	}
	return out
}
