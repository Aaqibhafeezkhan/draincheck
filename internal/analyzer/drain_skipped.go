package analyzer

import (
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
)

// skippedByDrain reports whether kubectl drain never evicts pod: a static
// (mirror) pod, which drain skips, or a pod controlled by a DaemonSet, which
// drain skips with --ignore-daemonsets and otherwise refuses to run for. No
// eviction request is made for such a pod, so no PodDisruptionBudget is ever
// consulted for it.
func skippedByDrain(pod corev1.Pod, owners *inventory.OwnerResolver) bool {
	return hasAnnotation(pod.Annotations, mirrorPodAnnotation) || owners.Resolve(pod).Kind == "DaemonSet"
}
