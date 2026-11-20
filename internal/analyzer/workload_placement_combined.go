package analyzer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
)

// WorkloadPlacementCombinedID is the rule ID reported by WorkloadPlacementCombined.
const WorkloadPlacementCombinedID = "WORKLOAD_PLACEMENT_COMBINED"

// WorkloadPlacementCombined reports a workload whose evicted pods cannot land
// on any node left, where neither cause explains it alone: taints rule out
// some nodes and labels (a nodeSelector or a required node affinity) rule out
// the others, or taints rule out every node and labels do too. The workload's
// controller recreates the pods and they stay Pending; the drain itself
// completes. It is RISK, not BLOCKED, and an estimate: it judges cordons,
// readiness, taints and required node labels, never capacity, and a constraint
// draincheck cannot evaluate counts as open (see unmatchedNodeLabels).
//
// The placement rules never report the same workload. WorkloadUntoleratedTaints
// states taints alone (labels leave a node open) and WorkloadUnmatchedLabels
// labels alone (taints leave a node open); this rule states what is left once
// no node has room: a workload for which every node left is ruled out and
// neither of those two holds. It does not defer to NoRemainingNodes (the
// per-workload advice is more specific than the run's), and like the label
// rule it is silent when no node is left at all. WorkloadUntoleratedTaints
// leaves a run in which every workload is stranded to NoRemainingNodes, so a
// workload stranded by taints alone has no placement finding then.
//
// One finding per workload (resolved with inventory.OwnerResolver), listing for
// each node left every taint and requirement that rules the workload out.
type WorkloadPlacementCombined struct{}

// ID implements Rule.
func (WorkloadPlacementCombined) ID() string { return WorkloadPlacementCombinedID }

// Evaluate implements Rule.
func (WorkloadPlacementCombined) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}
	moving := podsToMoveList(in.Snapshot, in.Targets, in.Drain)
	nodes := remainingNodes(in.Snapshot, in.Targets)
	if len(moving) == 0 || len(nodes) == 0 {
		return nil
	}

	var out []findings.Finding
	for _, g := range groupByWorkload(in.Snapshot, moving) {
		if !strandedByBothCauses(g, nodes) {
			continue
		}
		ev := placementCombinedEvidence(g, nodes)
		out = append(out, findings.Finding{
			RuleID:   WorkloadPlacementCombinedID,
			Severity: findings.Risk,
			Resource: findings.ResourceRef{Kind: g.owner.Kind, Namespace: g.owner.Namespace, Name: g.owner.Name},
			Evidence: ev,
			Case:     workloadPlacementCombinedCombined,
			Description: fmt.Sprintf(
				"%s %s/%s has %s on the target nodes and none can land on the %s left, because of taints and node labels together: %s. Fixing one cause is not enough.",
				g.owner.Kind, g.owner.Namespace, g.owner.Name, plural(len(ev.Pods), "pod"), plural(len(nodes), "node"), ruledOutList(ev.RuledOutNodes)),
		})
	}
	return out
}

// strandedByBothCauses says whether no node left has room for the group and
// the case is neither taints alone nor labels alone: with taints alone
// (labels leave a node open, taints rule out every node) WorkloadUntoleratedTaints
// speaks, with labels alone WorkloadUnmatchedLabels does.
func strandedByBothCauses(g *workloadGroup, nodes []corev1.Node) bool {
	var taintRoom, labelRoom bool
	for _, node := range nodes {
		p := placementOn(g.pods, node)
		if p.Room {
			return false
		}
		taintRoom = taintRoom || p.TaintRoom
		labelRoom = labelRoom || p.LabelRoom
	}
	return taintRoom == labelRoom
}

// placementCombinedEvidence builds the evidence for a workload that taints and
// labels together rule out of every node left. The lists are never nil, so the
// JSON has no null.
func placementCombinedEvidence(g *workloadGroup, nodes []corev1.Node) findings.WorkloadPlacementCombinedEvidence {
	ev := findings.WorkloadPlacementCombinedEvidence{OwnerKind: g.owner.Kind, OwnerName: g.owner.Name, RemainingNodes: len(nodes), Pods: g.strandedPods()}
	for _, node := range nodes {
		p := placementOn(g.pods, node)
		ev.RuledOutNodes = append(ev.RuledOutNodes, findings.RuledOutNode{
			Name:       node.Name,
			Taints:     append([]findings.NodeTaint{}, p.taints()...),
			Mismatches: append([]findings.LabelMismatch{}, p.labels()...),
		})
	}
	slices.SortFunc(ev.RuledOutNodes, func(a, b findings.RuledOutNode) int { return strings.Compare(a.Name, b.Name) })
	return ev
}

// ruledOutList names each node left with every reason that rules the workload
// out, taints first, for the description.
func ruledOutList(nodes []findings.RuledOutNode) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		var reasons []string
		for _, t := range n.Taints {
			reasons = append(reasons, "taint "+t.Key+":"+t.Effect)
		}
		for _, m := range n.Mismatches {
			reasons = append(reasons, describeMismatch(m))
		}
		parts[i] = n.Name + " (" + strings.Join(reasons, "; ") + ")"
	}
	return strings.Join(parts, ", ")
}
