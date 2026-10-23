package analyzer

import (
	"github.com/Veinar/draincheck/internal/findings"
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

// taintsAgainst reports whether any of the pods can land on node as far as
// taints go (room), and otherwise the distinct taints that keep them off,
// as (key, effect) pairs in the order the node lists them.
func taintsAgainst(pods []corev1.Pod, node corev1.Node) (blocking []findings.NodeTaint, room bool) {
	seen := map[findings.NodeTaint]bool{}
	for _, pod := range pods {
		untolerated := untoleratedTaints(pod, node)
		if len(untolerated) == 0 {
			return nil, true
		}
		for _, t := range untolerated {
			seen[findings.NodeTaint{Key: t.Key, Effect: string(t.Effect)}] = true
		}
	}
	for _, t := range node.Spec.Taints {
		nt := findings.NodeTaint{Key: t.Key, Effect: string(t.Effect)}
		if seen[nt] {
			blocking = append(blocking, nt)
			delete(seen, nt)
		}
	}
	return blocking, false
}
