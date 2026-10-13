package analyzer

import (
	"fmt"
	"slices"

	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// PDBDisruptionBlockedID is the rule ID reported by PDBDisruptionBlocked.
const PDBDisruptionBlockedID = "PDB_DISRUPTION_BLOCKED"

// PDBDisruptionBlocked reports pods on target nodes that are selected by a
// PodDisruptionBudget whose status allows zero disruptions, so the eviction
// API will refuse to evict them.
//
// One BLOCKED finding is emitted per pod and PDB pair, pointing at the pod,
// with the PDB as evidence. Pods in a terminal phase (Succeeded, Failed) are
// skipped because eviction does not consult budgets for them. A PDB with an
// invalid selector is skipped: it cannot be matched reliably.
type PDBDisruptionBlocked struct{}

// ID implements Rule.
func (PDBDisruptionBlocked) ID() string { return PDBDisruptionBlockedID }

// Evaluate implements Rule.
func (PDBDisruptionBlocked) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	var out []findings.Finding
	pdbs := in.Snapshot.PodDisruptionBudgets()
	for _, pod := range in.Snapshot.Pods() {
		if !slices.Contains(in.Targets, pod.Spec.NodeName) || terminal(pod) {
			continue
		}
		for _, pdb := range pdbs {
			if pdb.Namespace != pod.Namespace || pdb.Status.DisruptionsAllowed != 0 {
				continue
			}
			sel, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
			if err != nil || !sel.Matches(labels.Set(pod.Labels)) {
				continue
			}
			out = append(out, findings.Finding{
				RuleID:   PDBDisruptionBlockedID,
				Severity: findings.Blocked,
				Resource: findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
				Evidence: findings.PDBEvidence{
					Namespace:          pdb.Namespace,
					Name:               pdb.Name,
					CurrentHealthy:     pdb.Status.CurrentHealthy,
					DesiredHealthy:     pdb.Status.DesiredHealthy,
					DisruptionsAllowed: pdb.Status.DisruptionsAllowed,
				},
				Description: fmt.Sprintf(
					"PodDisruptionBudget %s/%s allows 0 disruptions (%d healthy, %d desired), so eviction of pod %s from node %s will be refused.",
					pdb.Namespace, pdb.Name, pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy, pod.Name, pod.Spec.NodeName),
				Remediation: "Add healthy replicas, or relax minAvailable/maxUnavailable on the PodDisruptionBudget, before draining.",
			})
		}
	}
	return out
}

func terminal(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
}
