package analyzer

import (
	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
)

// placementReasonKind says what rules a node out for a workload's pods.
// Taints are the only kind today; a new kind is a new constant and a new
// branch in placementOn, never a change to the rules that read the result.
type placementReasonKind string

// placementReasonTaint is a NoSchedule or NoExecute taint no pod tolerates.
const placementReasonTaint placementReasonKind = "taint"

// placementReason is one thing that rules a node out. Only the field of its
// Kind is set.
type placementReason struct {
	Kind  placementReasonKind
	Taint findings.NodeTaint
}

// nodePlacement is the answer of placementOn: whether any of the pods can land
// on the node and, when none can, every reason the node is ruled out.
type nodePlacement struct {
	Room    bool
	Reasons []placementReason
}

// placementOn reports whether any of the pods can land on node. When none can,
// the result lists the reasons, distinct and in the order the node lists them.
// With no pods there is nothing to land: no room, and no reason. Pure; it
// judges the node's taints and nothing else (no cordon, readiness, labels or
// capacity).
func placementOn(pods []corev1.Pod, node corev1.Node) nodePlacement {
	seen := map[findings.NodeTaint]bool{}
	for _, pod := range pods {
		untolerated := untoleratedTaints(pod, node)
		if len(untolerated) == 0 {
			return nodePlacement{Room: true}
		}
		for _, t := range untolerated {
			seen[findings.NodeTaint{Key: t.Key, Effect: string(t.Effect)}] = true
		}
	}
	var reasons []placementReason
	for _, t := range node.Spec.Taints {
		nt := findings.NodeTaint{Key: t.Key, Effect: string(t.Effect)}
		if seen[nt] {
			reasons = append(reasons, placementReason{Kind: placementReasonTaint, Taint: nt})
			delete(seen, nt)
		}
	}
	return nodePlacement{Reasons: reasons}
}

// taints returns the reasons that are taints, in order, for the evidence of the
// taint rules.
func (p nodePlacement) taints() []findings.NodeTaint {
	var out []findings.NodeTaint
	for _, r := range p.Reasons {
		if r.Kind == placementReasonTaint {
			out = append(out, r.Taint)
		}
	}
	return out
}
