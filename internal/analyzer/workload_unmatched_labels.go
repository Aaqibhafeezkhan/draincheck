package analyzer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
)

// WorkloadUnmatchedLabelsID is the rule ID reported by WorkloadUnmatchedLabels.
const WorkloadUnmatchedLabelsID = "WORKLOAD_UNMATCHED_NODE_LABELS"

// WorkloadUnmatchedLabels reports a workload whose evicted pods cannot land on
// any node left, because the labels of every node left rule them out (a
// nodeSelector or a required node affinity), while taints leave at least one
// node open. The workload's controller recreates the pods and they stay
// Pending; the drain itself completes. It is RISK, not BLOCKED, and an
// estimate: only required constraints count, and a constraint draincheck cannot
// evaluate counts as open (see unmatchedNodeLabels).
//
// The placement rules never report the same workload. This one states labels
// alone, WorkloadUntoleratedTaints states taints alone (when labels leave a
// node open), and a workload that needs both causes to explain it, or that
// each of them rules out, is left to the combined rule. Like the taint rule it
// judges cordons and readiness for the nodes left (see remainingNodes), and it
// is silent when no node is left at all (NoRemainingNodes states that).
//
// One finding per workload (resolved with inventory.OwnerResolver), listing for
// each node left the requirements the workload does not meet.
type WorkloadUnmatchedLabels struct{}

// ID implements Rule.
func (WorkloadUnmatchedLabels) ID() string { return WorkloadUnmatchedLabelsID }

// Evaluate implements Rule.
func (WorkloadUnmatchedLabels) Evaluate(in Input) []findings.Finding {
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
		room := g.roomOf(nodes)
		if room.label || !room.taint {
			continue
		}
		ev := unmatchedLabelsEvidence(g, nodes)
		out = append(out, findings.Finding{
			RuleID:   WorkloadUnmatchedLabelsID,
			Severity: findings.Risk,
			Resource: findings.ResourceRef{Kind: g.owner.Kind, Namespace: g.owner.Namespace, Name: g.owner.Name},
			Evidence: ev,
			Case:     workloadUnmatchedLabelsUnmatched,
			Description: fmt.Sprintf(
				"%s %s/%s has %s on the target nodes and none can land on the %s left, because of node labels: %s.",
				g.owner.Kind, g.owner.Namespace, g.owner.Name, plural(len(ev.Pods), "pod"), plural(len(nodes), "node"), mismatchList(ev.MismatchedNodes)),
		})
	}
	return out
}

// unmatchedLabelsEvidence builds the evidence for a workload that labels rule
// out of every node left.
func unmatchedLabelsEvidence(g *workloadGroup, nodes []corev1.Node) findings.WorkloadUnmatchedLabelsEvidence {
	ev := findings.WorkloadUnmatchedLabelsEvidence{OwnerKind: g.owner.Kind, OwnerName: g.owner.Name, RemainingNodes: len(nodes), Pods: g.strandedPods()}
	for _, node := range nodes {
		ev.MismatchedNodes = append(ev.MismatchedNodes, findings.MismatchedNode{Name: node.Name, Mismatches: placementOn(g.pods, node).labels()})
	}
	slices.SortFunc(ev.MismatchedNodes, func(a, b findings.MismatchedNode) int { return strings.Compare(a.Name, b.Name) })
	return ev
}

// mismatchList names each node left with the requirements that rule the
// workload out, for the description.
func mismatchList(nodes []findings.MismatchedNode) string {
	parts := make([]string, len(nodes))
	for i, n := range nodes {
		reqs := make([]string, len(n.Mismatches))
		for j, m := range n.Mismatches {
			reqs[j] = describeMismatch(m)
		}
		parts[i] = n.Name + " (" + strings.Join(reqs, "; ") + ")"
	}
	return strings.Join(parts, ", ")
}

// describeMismatch says what one requirement wants and what the node has.
func describeMismatch(m findings.LabelMismatch) string {
	var want string
	switch m.Source {
	case findings.LabelSourceNodeSelector:
		want = fmt.Sprintf("nodeSelector %s=%s", m.Key, strings.Join(m.Values, ","))
	case findings.LabelSourceNodeAffinityField:
		want = fmt.Sprintf("required node affinity field %s %s", m.Key, operatorWants(m))
	default:
		want = fmt.Sprintf("required node affinity %s %s", m.Key, operatorWants(m))
	}
	switch {
	case m.Source == findings.LabelSourceNodeAffinityField:
		return want + fmt.Sprintf(", node is %s", m.NodeValue)
	case m.NodeHasKey:
		return want + fmt.Sprintf(", node has %s=%s", m.Key, m.NodeValue)
	default:
		return want + fmt.Sprintf(", node has no %s", m.Key)
	}
}

// operatorWants renders an operator and its values: "In [a b]", "Exists".
func operatorWants(m findings.LabelMismatch) string {
	if len(m.Values) == 0 {
		return m.Operator
	}
	return m.Operator + " [" + strings.Join(m.Values, " ") + "]"
}
