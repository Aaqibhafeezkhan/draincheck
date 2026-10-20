package analyzer

import (
	"fmt"

	"github.com/Veinar/draincheck/internal/findings"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PDBSyncFailedID is the rule ID reported by PDBSyncFailed.
const PDBSyncFailedID = "PDB_SYNC_FAILED"

// PDBSyncFailed reports PodDisruptionBudgets whose disruption controller could
// not compute the budget while they select pods on target nodes: the
// DisruptionAllowed condition has the reason SyncFailed, for example because
// the pods' owner has no scale subresource. The controller then records that no
// disruption is allowed, which says nothing about the real budget, so
// draincheck cannot tell whether the drain will be refused. A false BLOCKED
// costs more trust than a missed one, so the answer is UNKNOWN.
//
// This finding replaces the one PDB_DISRUPTION_BLOCKED would have given for the
// same budget. It is reported whatever number the failed status holds. A budget
// that is stale as well (see PDBStaleStatus) is left to that rule, so one
// budget is one finding. One UNKNOWN finding is emitted per budget, pointing at
// the budget, with the pods and nodes it selects, the numbers as last reported
// and the condition in typed evidence. The pods counted are the same ones
// PDB_DISRUPTION_BLOCKED counts (see selectedPDBs).
type PDBSyncFailed struct{}

// ID implements Rule.
func (PDBSyncFailed) ID() string { return PDBSyncFailedID }

// Evaluate implements Rule.
func (PDBSyncFailed) Evaluate(in Input) []findings.Finding {
	var out []findings.Finding
	for _, sel := range selectedPDBs(in) {
		pdb := sel.pdb
		cond, failed := pdbSyncFailure(pdb)
		if !failed || pdbStale(pdb) {
			continue
		}
		cause := ""
		if cond.Message != "" {
			cause = " (" + cond.Message + ")"
		}
		out = append(out, findings.Finding{
			RuleID:   PDBSyncFailedID,
			Severity: findings.Unknown,
			Resource: sel.resource(),
			Evidence: findings.PDBSyncFailedEvidence{
				PDBEvidence: sel.evidence(),
				Reason:      cond.Reason,
				Message:     cond.Message,
			},
			Description: fmt.Sprintf(
				"The disruption controller could not compute PodDisruptionBudget %s/%s%s, so the numbers in its status cannot be trusted and draincheck cannot tell whether eviction of its %s on %s will be refused.",
				pdb.Namespace, pdb.Name, cause,
				plural(len(sel.pods), "pod"), plural(len(sel.nodes), "target node")),
			Remediation: "Fix what the disruption controller reports, usually a pod owner that has no scale subresource or a selector it cannot resolve, then run draincheck again. Until then, drain with care: the budget cannot be relied on either way.",
		})
	}
	return out
}

// pdbSyncFailure returns the DisruptionAllowed condition of pdb and true when
// its reason is SyncFailed: the disruption controller could not compute the
// budget, so the numbers in the status cannot be trusted. A budget at 0 for
// another reason (InsufficientPods) is a plain budget at 0, not a failure.
func pdbSyncFailure(pdb policyv1.PodDisruptionBudget) (metav1.Condition, bool) {
	for _, cond := range pdb.Status.Conditions {
		if cond.Type == policyv1.DisruptionAllowedCondition && cond.Reason == policyv1.SyncFailedReason {
			return cond, true
		}
	}
	return metav1.Condition{}, false
}
