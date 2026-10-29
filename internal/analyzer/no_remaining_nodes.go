package analyzer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
)

// NoRemainingNodesID is the rule ID reported by NoRemainingNodes.
const NoRemainingNodesID = "NO_REMAINING_NODES"

// NoRemainingNodes reports a drain that leaves no node able to take the
// evicted pods: every node outside the target set is cordoned or not Ready
// (see unschedulable), or carries a NoSchedule or NoExecute taint that none of
// the moving pods tolerates (see untoleratedTaints), or there is no node
// outside the target set at all.
// kubectl drain still completes, since it only evicts; the pods it moves are
// recreated by their controllers and stay Pending. It is RISK, not BLOCKED.
//
// Like NodeNotSchedulable it fires only when the drain moves something (see
// podsToMove). When the nodes left are all unschedulable, NodeNotSchedulable
// names them and this rule states the consequence. It is silent as soon as one
// moving pod can land on one node left, so room for some pods and none for
// others is not reported; a per-workload variant is planned. Affinity,
// selectors and capacity are not looked at.
//
// One finding covers the run; its resource is the target node names, sorted
// and joined by commas.
type NoRemainingNodes struct{}

// ID implements Rule.
func (NoRemainingNodes) ID() string { return NoRemainingNodesID }

// Evaluate implements Rule.
func (NoRemainingNodes) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	moving := podsToMoveList(in.Snapshot, in.Targets, in.Drain)
	if len(moving) == 0 {
		return nil
	}

	nodes := in.Snapshot.Nodes()
	var tainted []findings.TaintedNode
	for _, node := range nodes {
		if slices.Contains(in.Targets, node.Name) {
			continue
		}
		if _, blocked := unschedulable(node); blocked {
			continue
		}
		taints, room := taintsAgainst(moving, node)
		if room {
			return nil
		}
		tainted = append(tainted, findings.TaintedNode{Name: node.Name, Taints: taints})
	}
	slices.SortFunc(tainted, func(a, b findings.TaintedNode) int { return strings.Compare(a.Name, b.Name) })

	targets := slices.Sorted(slices.Values(in.Targets))
	return []findings.Finding{{
		RuleID:   NoRemainingNodesID,
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: "Node", Name: strings.Join(targets, ",")},
		Evidence: findings.NoRemainingNodesEvidence{
			PodsToMove:           len(moving),
			TaintedNodes:         tainted,
			TargetNodes:          len(targets),
			TotalNodes:           len(nodes),
			RemainingSchedulable: 0,
		},
		Description: fmt.Sprintf(
			"The drain targets %d of %s and no node is left that takes new pods. %sThe %s it evicts are recreated by their controllers and stay Pending until a node can take them. kubectl drain only evicts, so it completes.",
			len(targets), plural(len(nodes), "node"), taintClause(tainted), plural(len(moving), "pod")),
		Remediation: "Confirm with `kubectl get nodes`: no node outside the drain is Ready without SchedulingDisabled. Then, safest first: add a node, or repair or uncordon one, before draining; or drain fewer nodes at once. The drain itself completes either way; afterwards `kubectl get pods -A --field-selector=status.phase=Pending` shows what is waiting. If the description names a taint on a node, `kubectl describe node <node>` shows it under Taints; remove it, or give the workload a matching toleration, only if the node is meant to run it. Cordons, readiness and taints are checked, not affinity or capacity.",
	}}
}

// taintClause names the nodes that are left and Ready but that a taint rules
// out, as a sentence for the description. It is empty when no taint is involved.
func taintClause(tainted []findings.TaintedNode) string {
	if len(tainted) == 0 {
		return ""
	}
	parts := make([]string, len(tainted))
	for i, n := range tainted {
		keys := make([]string, len(n.Taints))
		for j, t := range n.Taints {
			keys[j] = t.Key + ":" + t.Effect
		}
		parts[i] = n.Name + " (taint " + strings.Join(keys, ", ") + ")"
	}
	return "Ready nodes left but ruled out by taints that none of the moving pods tolerates: " + strings.Join(parts, "; ") + ". "
}
