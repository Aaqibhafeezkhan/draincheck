package analyzer

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
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
type PDBDisruptionBlocked struct{}

// ID implements Rule.
func (PDBDisruptionBlocked) ID() string { return PDBDisruptionBlockedID }

// Evaluate implements Rule.
func (PDBDisruptionBlocked) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	owners := inventory.NewOwnerResolver(in.Snapshot)
	var evicted []corev1.Pod // pods on the target nodes that kubectl drain would evict
	for _, pod := range in.Snapshot.Pods() {
		if slices.Contains(in.Targets, pod.Spec.NodeName) && !terminal(pod) && !skippedByDrain(pod, owners) {
			evicted = append(evicted, pod)
		}
	}

	var out []findings.Finding
	for _, pdb := range in.Snapshot.PodDisruptionBudgets() {
		if pdb.Status.DisruptionsAllowed != 0 {
			continue
		}
		sel, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			continue
		}

		var pods []findings.PDBPod
		for _, pod := range evicted {
			if pod.Namespace == pdb.Namespace && sel.Matches(labels.Set(pod.Labels)) {
				pods = append(pods, findings.PDBPod{Namespace: pod.Namespace, Name: pod.Name, Node: pod.Spec.NodeName})
			}
		}
		if len(pods) == 0 {
			continue
		}
		slices.SortFunc(pods, func(a, b findings.PDBPod) int {
			return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name), cmp.Compare(a.Node, b.Node))
		})
		nodes := make([]string, len(pods))
		for i, p := range pods {
			nodes[i] = p.Node
		}
		slices.Sort(nodes)
		nodes = slices.Compact(nodes)

		out = append(out, findings.Finding{
			RuleID:   PDBDisruptionBlockedID,
			Severity: findings.Blocked,
			Resource: findings.ResourceRef{Kind: "PodDisruptionBudget", Namespace: pdb.Namespace, Name: pdb.Name},
			Evidence: findings.PDBEvidence{
				Namespace:          pdb.Namespace,
				Name:               pdb.Name,
				CurrentHealthy:     pdb.Status.CurrentHealthy,
				DesiredHealthy:     pdb.Status.DesiredHealthy,
				DisruptionsAllowed: pdb.Status.DisruptionsAllowed,
				Nodes:              nodes,
				Pods:               pods,
			},
			Description: fmt.Sprintf(
				"PodDisruptionBudget %s/%s allows 0 disruptions (%d healthy, %d desired), so eviction of its %s on %s will be refused.",
				pdb.Namespace, pdb.Name, pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy,
				plural(len(pods), "pod"), plural(len(nodes), "target node")),
			Remediation: "Add healthy replicas, or relax minAvailable/maxUnavailable on the PodDisruptionBudget, before draining.",
		})
	}
	return out
}

func terminal(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
}
