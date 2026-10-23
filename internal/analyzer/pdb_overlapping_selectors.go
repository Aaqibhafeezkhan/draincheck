package analyzer

import (
	"fmt"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
)

// PDBOverlappingSelectorsID is the rule ID reported by PDBOverlappingSelectors.
const PDBOverlappingSelectorsID = "PDB_OVERLAPPING_SELECTORS"

// PDBOverlappingSelectors reports pods on target nodes that are selected by
// more than one PodDisruptionBudget. The eviction API does not support that
// and rejects the eviction with an error, whatever the budgets allow.
//
// One BLOCKED finding is emitted per pod, with every matching PDB as
// evidence. Pods that kubectl drain never evicts are skipped (DaemonSet pods
// and static pods, see podDrain.consultsBudget), as are pods in a terminal phase
// (Succeeded, Failed), because eviction does not consult budgets for them. A
// PDB with an invalid selector is skipped: it cannot be matched reliably (see
// overlaps).
//
// The budgets are listed in the evidence with the numbers their status reports.
// The disruption controller chooses one of the budgets that select a pod when
// the pod changes, and the others can show numbers that are out of date (see
// overlappedPDBs): PDBDisruptionBlocked and PDBBatchExceedsBudget do not read
// them, and this is where they stay visible.
type PDBOverlappingSelectors struct{}

// ID implements Rule.
func (PDBOverlappingSelectors) ID() string { return PDBOverlappingSelectorsID }

// Evaluate implements Rule.
func (PDBOverlappingSelectors) Evaluate(in Input) []findings.Finding {
	var out []findings.Finding
	for _, o := range overlaps(in) {
		pod := o.pod
		names := make([]string, len(o.pdbs))
		budgets := make([]findings.OverlappingPDB, len(o.pdbs))
		numbers := make([]string, len(o.pdbs))
		for i, pdb := range o.pdbs {
			names[i] = pdb.Name
			budgets[i] = findings.OverlappingPDB{
				Name:               pdb.Name,
				CurrentHealthy:     pdb.Status.CurrentHealthy,
				DesiredHealthy:     pdb.Status.DesiredHealthy,
				DisruptionsAllowed: pdb.Status.DisruptionsAllowed,
			}
			numbers[i] = fmt.Sprintf("%s %d healthy, %d desired, %d allowed", pdb.Name, pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy, pdb.Status.DisruptionsAllowed)
		}
		out = append(out, findings.Finding{
			RuleID:   PDBOverlappingSelectorsID,
			Severity: findings.Blocked,
			Resource: findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
			Evidence: findings.OverlappingPDBsEvidence{Namespace: pod.Namespace, PDBs: names, Budgets: budgets},
			Description: fmt.Sprintf(
				"Pod %s on node %s is selected by %d PodDisruptionBudgets in namespace %s (%s); the eviction API does not support more than one and will refuse the eviction. kubectl drain reports this error for the pod at once and does not retry it. The disruption controller chooses one of them arbitrarily when a pod changes, so their numbers can be out of date and are not used for other findings; as reported they are %s.",
				pod.Name, pod.Spec.NodeName, len(names), pod.Namespace, strings.Join(names, ", "), strings.Join(numbers, "; ")),
			Remediation: "Confirm with `kubectl get pod -n <namespace> <pod> --show-labels` and `kubectl describe pdb -n <namespace> <pdb>` for each budget named in the finding (the selector of each). Then narrow the PodDisruptionBudget selectors so that each pod is covered by exactly one budget, or merge the budgets, before draining. Waiting does not help: the eviction API refuses a pod covered by several budgets whatever they allow. Run draincheck again afterwards: the budgets are checked on their own once they no longer overlap.",
		})
	}
	return out
}
