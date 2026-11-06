package analyzer

import (
	"fmt"

	"github.com/Veinar/draincheck/internal/findings"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// PDBUnsatisfiableID is the rule ID reported by PDBUnsatisfiable.
const PDBUnsatisfiableID = "PDB_UNSATISFIABLE"

// PDBUnsatisfiable reports PodDisruptionBudgets that can never allow a
// disruption while they select pods on target nodes: status.desiredHealthy is
// not below status.expectedPods, which is minAvailable at or above the replica
// count or maxUnavailable 0. However healthy the workload is, the eviction API
// refuses the pods, so kubectl drain retries until --timeout (infinite by
// default) and does not complete. Only editing the budget or scaling the
// workload clears it. BLOCKED, one finding per budget, pointing at it.
//
// This rule and PDBDisruptionBlocked split the budgets at 0 between them
// through unsatisfiableBudget, so no budget gets both: this one takes the
// budgets that never allow a disruption, PDBDisruptionBlocked keeps the ones at
// 0 that may clear (a pod that is not Ready, a rollout) and the ones it cannot
// judge (stale, sync failed, operator-managed, overlapping, unknown policy).
//
// maxUnavailable 0 is often deliberate; the description says so. Pods and
// budgets are selected as for PDBDisruptionBlocked (selectedPDBs), so
// DaemonSet and static pods, finished pods and, with --disable-eviction, every
// pod are not counted.
type PDBUnsatisfiable struct{}

// ID implements Rule.
func (PDBUnsatisfiable) ID() string { return PDBUnsatisfiableID }

// Evaluate implements Rule.
func (PDBUnsatisfiable) Evaluate(in Input) []findings.Finding {
	var out []findings.Finding
	for _, sel := range selectedPDBs(in) {
		if !unsatisfiableBudget(sel) {
			continue
		}
		pdb := sel.pdb
		minAvail, maxUnavail := intOrPercent(pdb.Spec.MinAvailable), intOrPercent(pdb.Spec.MaxUnavailable)

		var cause string
		if maxUnavail != "" && budgetIsZero(pdb.Spec.MaxUnavailable) {
			cause = "maxUnavailable is 0, which is often deliberate (a workload that must never lose a pod), but it means no pod can be evicted."
		} else {
			cause = fmt.Sprintf("minAvailable %s asks for %d healthy pods and the workload has %d expected pods, so there is never one to spare.",
				minAvail, pdb.Status.DesiredHealthy, pdb.Status.ExpectedPods)
		}
		out = append(out, findings.Finding{
			RuleID:   PDBUnsatisfiableID,
			Severity: findings.Blocked,
			Resource: sel.resource(),
			Evidence: findings.PDBUnsatisfiableEvidence{
				Namespace: pdb.Namespace, Name: pdb.Name,
				ExpectedPods: pdb.Status.ExpectedPods, DesiredHealthy: pdb.Status.DesiredHealthy,
				MinAvailable: minAvail, MaxUnavailable: maxUnavail,
				Nodes: sel.nodes, Pods: sel.pods,
			},
			Case: pdbUnsatisfiableNeverAllows,
			Description: fmt.Sprintf(
				"PodDisruptionBudget %s/%s can never allow a disruption: it needs %d healthy of %d expected pods. %s It selects %s on %s.",
				pdb.Namespace, pdb.Name, pdb.Status.DesiredHealthy, pdb.Status.ExpectedPods, cause,
				plural(len(sel.pods), "pod"), plural(len(sel.nodes), "target node")),
		})
	}
	return out
}

// unsatisfiableBudget reports whether sel is a budget PDBUnsatisfiable
// reports: one that can never allow a disruption (desiredHealthy not below
// expectedPods, with at least one expected pod) and whose numbers can be
// trusted. PDBDisruptionBlocked skips exactly these budgets, so each is
// reported once. A budget with a stale status, a failed sync, an operator-managed pod, an
// overlapping budget or an unknown policy is left to the rules that report those
// states.
func unsatisfiableBudget(sel pdbSelection) bool {
	pdb := sel.pdb
	if sel.overlapped || !sel.policyKnown || len(sel.operators) > 0 || pdbStale(pdb) {
		return false
	}
	if _, failed := pdbSyncFailure(pdb); failed {
		return false
	}
	return pdb.Status.ExpectedPods > 0 && pdb.Status.DesiredHealthy >= pdb.Status.ExpectedPods && pdb.Status.DisruptionsAllowed == 0
}

// intOrPercent renders spec.minAvailable or spec.maxUnavailable as configured,
// empty when unset.
func intOrPercent(v *intstr.IntOrString) string {
	if v == nil {
		return ""
	}
	return v.String()
}

// budgetIsZero reports whether v is 0 or 0%.
func budgetIsZero(v *intstr.IntOrString) bool {
	return v != nil && (v.String() == "0" || v.String() == "0%")
}
