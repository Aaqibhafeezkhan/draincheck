package analyzer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// PDBOverlappingSelectorsID is the rule ID reported by PDBOverlappingSelectors.
const PDBOverlappingSelectorsID = "PDB_OVERLAPPING_SELECTORS"

// PDBOverlappingSelectors reports pods on target nodes that are selected by
// more than one PodDisruptionBudget. The eviction API does not support that
// and rejects the eviction with an error, whatever the budgets allow.
//
// One BLOCKED finding is emitted per pod, with every matching PDB as
// evidence. Pods in a terminal phase (Succeeded, Failed) are skipped because
// eviction does not consult budgets for them. A PDB with an invalid selector
// is skipped: it cannot be matched reliably.
type PDBOverlappingSelectors struct{}

// ID implements Rule.
func (PDBOverlappingSelectors) ID() string { return PDBOverlappingSelectorsID }

// Evaluate implements Rule.
func (PDBOverlappingSelectors) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	var out []findings.Finding
	pdbs := in.Snapshot.PodDisruptionBudgets()
	for _, pod := range in.Snapshot.Pods() {
		if !slices.Contains(in.Targets, pod.Spec.NodeName) || terminal(pod) {
			continue
		}
		var matched []string
		for _, pdb := range pdbs {
			if pdb.Namespace != pod.Namespace {
				continue
			}
			sel, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
			if err != nil || !sel.Matches(labels.Set(pod.Labels)) {
				continue
			}
			matched = append(matched, pdb.Name)
		}
		if len(matched) < 2 {
			continue
		}
		slices.Sort(matched)
		out = append(out, findings.Finding{
			RuleID:   PDBOverlappingSelectorsID,
			Severity: findings.Blocked,
			Resource: findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
			Evidence: findings.OverlappingPDBsEvidence{Namespace: pod.Namespace, PDBs: matched},
			Description: fmt.Sprintf(
				"Pod %s on node %s is selected by %d PodDisruptionBudgets in namespace %s (%s); the eviction API does not support more than one and will refuse the eviction.",
				pod.Name, pod.Spec.NodeName, len(matched), pod.Namespace, strings.Join(matched, ", ")),
			Remediation: "Narrow the PodDisruptionBudget selectors so that each pod is covered by exactly one budget, or merge the budgets, before draining.",
		})
	}
	return out
}
