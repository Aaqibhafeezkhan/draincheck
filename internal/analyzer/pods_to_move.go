package analyzer

import (
	"slices"

	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
)

// podsToMove counts the pods on the target nodes that a drain evicts and that
// a controller recreates elsewhere (see podsToMoveList).
func podsToMove(s *inventory.Snapshot, targets []string) int {
	return len(podsToMoveList(s, targets))
}

// podsToMoveList returns the pods on the target nodes that a drain evicts and
// that a controller recreates elsewhere (fateEvictedRescheduled). DaemonSet,
// static, finished, already terminating and controller-less pods need no new
// home, so they are not returned.
func podsToMoveList(s *inventory.Snapshot, targets []string) []corev1.Pod {
	owners := inventory.NewOwnerResolver(s)
	var out []corev1.Pod
	for _, pod := range s.Pods() {
		if slices.Contains(targets, pod.Spec.NodeName) && classifyPod(pod, owners).fate() == fateEvictedRescheduled {
			out = append(out, pod)
		}
	}
	return out
}
