package analyzer

import (
	"slices"

	"github.com/Veinar/draincheck/internal/inventory"
)

// podsToMove counts the pods on the target nodes that a drain evicts and that
// a controller recreates elsewhere. DaemonSet, static, terminal and
// controller-less pods need no new home (see skippedByDrain and terminal), so
// they are not counted.
func podsToMove(s *inventory.Snapshot, targets []string) int {
	owners := inventory.NewOwnerResolver(s)
	n := 0
	for _, pod := range s.Pods() {
		if slices.Contains(targets, pod.Spec.NodeName) && !terminal(pod) &&
			!skippedByDrain(pod, owners) && !owners.Resolve(pod).Unmanaged() {
			n++
		}
	}
	return n
}
