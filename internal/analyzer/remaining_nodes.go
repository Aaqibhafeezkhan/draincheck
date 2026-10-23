package analyzer

import (
	"slices"

	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
)

// remainingNodes returns the nodes outside the target set that take new pods
// as far as cordons and readiness go (see unschedulable). Taints are judged
// per pod (see untoleratedTaints), not here.
func remainingNodes(s *inventory.Snapshot, targets []string) []corev1.Node {
	var out []corev1.Node
	for _, node := range s.Nodes() {
		if slices.Contains(targets, node.Name) {
			continue
		}
		if _, blocked := unschedulable(node); blocked {
			continue
		}
		out = append(out, node)
	}
	return out
}
