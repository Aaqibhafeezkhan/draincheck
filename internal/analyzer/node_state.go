package analyzer

import (
	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
)

// unschedulable reports whether pods cannot be scheduled on node: it is
// cordoned (spec.unschedulable), or its Ready condition is False or Unknown.
// A node that reports no Ready condition is not flagged for that: every real
// node reports one, so its absence says nothing, and a false RISK costs more
// than a missed one. When it reports true, the returned value describes the node for evidence.
//
// Taints and pressure conditions are not looked at; placement rules own them.
func unschedulable(node corev1.Node) (findings.UnschedulableNode, bool) {
	state := findings.UnschedulableNode{Name: node.Name, Cordoned: node.Spec.Unschedulable}
	notReady := false
	for _, cond := range node.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			state.ReadyStatus = string(cond.Status)
			notReady = cond.Status != corev1.ConditionTrue
			break
		}
	}
	if !state.Cordoned && !notReady {
		return findings.UnschedulableNode{}, false
	}
	return state, true
}
