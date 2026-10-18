package analyzer

import (
	"fmt"

	"github.com/Veinar/draincheck/internal/findings"
	policyv1 "k8s.io/api/policy/v1"
)

// PDBStaleStatusID is the rule ID reported by PDBStaleStatus.
const PDBStaleStatusID = "PDB_STALE_STATUS"

// PDBStaleStatus reports PodDisruptionBudgets whose status has not caught up
// with their spec (status.observedGeneration differs from metadata.generation)
// while they select pods on target nodes. The numbers in such a status,
// disruptionsAllowed included, may describe a spec that no longer exists, so
// draincheck cannot say whether the drain will be refused. A false BLOCKED
// costs more trust than a missed one, so the answer is UNKNOWN.
//
// This finding replaces the one PDB_DISRUPTION_BLOCKED would have given for the
// same budget. It is reported whatever the stale status allows: a stale number
// that allows a disruption cannot be trusted either. One UNKNOWN finding is
// emitted per budget, pointing at the budget, with the pods and nodes it
// selects and the numbers as last reported in typed evidence. The pods counted
// are the same ones PDB_DISRUPTION_BLOCKED counts (see selectedPDBs).
type PDBStaleStatus struct{}

// ID implements Rule.
func (PDBStaleStatus) ID() string { return PDBStaleStatusID }

// Evaluate implements Rule.
func (PDBStaleStatus) Evaluate(in Input) []findings.Finding {
	var out []findings.Finding
	for _, sel := range selectedPDBs(in) {
		pdb := sel.pdb
		if !pdbStale(pdb) {
			continue
		}
		out = append(out, findings.Finding{
			RuleID:   PDBStaleStatusID,
			Severity: findings.Unknown,
			Resource: sel.resource(),
			Evidence: findings.PDBStaleStatusEvidence{
				PDBEvidence:        sel.evidence(),
				ObservedGeneration: pdb.Status.ObservedGeneration,
				Generation:         pdb.Generation,
			},
			Description: fmt.Sprintf(
				"PodDisruptionBudget %s/%s has not been reconciled (status.observedGeneration %d, metadata.generation %d), so the numbers in its status are out of date and draincheck cannot tell whether eviction of its %s on %s will be refused.",
				pdb.Namespace, pdb.Name, pdb.Status.ObservedGeneration, pdb.Generation,
				plural(len(sel.pods), "pod"), plural(len(sel.nodes), "target node")),
			Remediation: "Wait for the disruption controller to update the PodDisruptionBudget status, then run draincheck again. If it stays out of date, check that kube-controller-manager is healthy.",
		})
	}
	return out
}

// pdbStale reports whether the status of pdb describes a different spec
// generation than the current one, so its numbers cannot be trusted. Both are
// 0 when a cluster does not set them, which counts as reconciled.
func pdbStale(pdb policyv1.PodDisruptionBudget) bool {
	return pdb.Status.ObservedGeneration != pdb.Generation
}
