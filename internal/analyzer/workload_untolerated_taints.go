package analyzer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
)

// WorkloadUntoleratedTaintsID is the rule ID reported by WorkloadUntoleratedTaints.
const WorkloadUntoleratedTaintsID = "WORKLOAD_UNTOLERATED_TAINTS"

// WorkloadUntoleratedTaints reports a workload whose evicted pods cannot land on any node
// left, because of taints, while pods of other workloads can. The workload's
// controller recreates the pods and they stay Pending; the drain itself
// completes. It is RISK, not BLOCKED.
//
// NoRemainingNodes states the case where no moving pod can land anywhere, for
// the whole run, and the case where no node is left at all, so this rule stays
// silent there. One finding per workload (resolved with inventory.OwnerResolver),
// listing the nodes left and the taints the workload does not tolerate.
//
// It states taints alone: it is silent when the workload's labels (nodeSelector,
// required node affinity) rule out every node left too, because adding a
// toleration would not help; WorkloadUnmatchedLabels and the combined case
// own that. Capacity is never looked at.
type WorkloadUntoleratedTaints struct{}

// ID implements Rule.
func (WorkloadUntoleratedTaints) ID() string { return WorkloadUntoleratedTaintsID }

// Evaluate implements Rule.
func (WorkloadUntoleratedTaints) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}
	moving := podsToMoveList(in.Snapshot, in.Targets, in.Drain)
	nodes := remainingNodes(in.Snapshot, in.Targets)
	if len(moving) == 0 || len(nodes) == 0 {
		return nil
	}

	groups := groupByWorkload(in.Snapshot, moving)
	var stranded []*workloadGroup
	for _, g := range groups {
		if !g.roomOf(nodes).taint {
			stranded = append(stranded, g)
		}
	}
	// Every workload stranded: the run has no room, which NoRemainingNodes states.
	if len(stranded) == 0 || len(stranded) == len(groups) {
		return nil
	}

	out := make([]findings.Finding, 0, len(stranded))
	for _, g := range stranded {
		// A workload whose labels rule out every node too is ruled out by both
		// causes together, which neither this rule nor WorkloadUnmatchedLabels
		// states; telling it to tolerate a taint would not help.
		if !g.roomOf(nodes).label {
			continue
		}
		ev := untoleratedEvidence(g, nodes)
		out = append(out, findings.Finding{
			RuleID:   WorkloadUntoleratedTaintsID,
			Severity: findings.Risk,
			Resource: findings.ResourceRef{Kind: g.owner.Kind, Namespace: g.owner.Namespace, Name: g.owner.Name},
			Evidence: ev,
			Case:     workloadUntoleratedTaintsUntolerated,
			Description: fmt.Sprintf(
				"%s %s/%s has %s on the target nodes and none can land on the %s left: %s. Other pods of this drain can.",
				g.owner.Kind, g.owner.Namespace, g.owner.Name, plural(len(ev.Pods), "pod"), plural(len(nodes), "node"), taintList(ev.TaintedNodes)),
		})
	}
	return out
}

// untoleratedEvidence builds the evidence for a workload that taints rule out of every node left.
func untoleratedEvidence(g *workloadGroup, nodes []corev1.Node) findings.WorkloadUntoleratedTaintsEvidence {
	ev := findings.WorkloadUntoleratedTaintsEvidence{OwnerKind: g.owner.Kind, OwnerName: g.owner.Name, RemainingNodes: len(nodes)}
	ev.Pods = g.strandedPods()
	for _, node := range nodes {
		ev.TaintedNodes = append(ev.TaintedNodes, findings.TaintedNode{Name: node.Name, Taints: placementOn(g.pods, node).taints()})
	}
	slices.SortFunc(ev.TaintedNodes, func(a, b findings.TaintedNode) int { return strings.Compare(a.Name, b.Name) })
	return ev
}

// taintList names each node left with the taints that rule the workload out.
func taintList(nodes []findings.TaintedNode) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		keys := make([]string, len(n.Taints))
		for j, t := range n.Taints {
			keys[j] = t.Key + ":" + t.Effect
		}
		parts[i] = n.Name + " (taint " + strings.Join(keys, ", ") + ")"
	}
	return strings.Join(parts, "; ")
}
