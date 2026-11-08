package analyzer

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// untoleratedTaints returns the taints of node that keep pod off it: the
// NoSchedule and NoExecute ones that no toleration of the pod matches, in the
// order the node lists them. PreferNoSchedule never removes room, so it is not
// returned. Matching is the scheduler's own (Toleration.ToleratesTaint):
// operator Equal or Exists, an empty key with Exists matches every key, an
// empty effect matches every effect, and tolerationSeconds is not consulted. The Lt and Gt operators are
// always honoured: the API server only stores them when the cluster allows them.
func untoleratedTaints(pod corev1.Pod, node corev1.Node) []corev1.Taint {
	var out []corev1.Taint
	for _, taint := range node.Spec.Taints {
		if taint.Effect != corev1.TaintEffectNoSchedule && taint.Effect != corev1.TaintEffectNoExecute {
			continue
		}
		tolerated := false
		for i := range pod.Spec.Tolerations {
			if pod.Spec.Tolerations[i].ToleratesTaint(klog.Background(), &taint, true) {
				tolerated = true
				break
			}
		}
		if !tolerated {
			out = append(out, taint)
		}
	}
	return out
}
