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
// (see unschedulable), or there is no node outside the target set at all.
// kubectl drain still completes, since it only evicts; the pods it moves are
// recreated by their controllers and stay Pending. It is RISK, not BLOCKED.
//
// Like NodeNotSchedulable it fires only when the drain moves something (see
// podsToMove). When the nodes left are all unschedulable, NodeNotSchedulable
// names them and this rule states the consequence. Nothing here looks at
// taints, affinity or capacity, so it never claims more than "no node takes
// pods"; a per-workload variant is planned.
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

	moving := podsToMove(in.Snapshot, in.Targets)
	if moving == 0 {
		return nil
	}

	nodes := in.Snapshot.Nodes()
	for _, node := range nodes {
		if slices.Contains(in.Targets, node.Name) {
			continue
		}
		if _, blocked := unschedulable(node); !blocked {
			return nil
		}
	}

	targets := slices.Sorted(slices.Values(in.Targets))
	return []findings.Finding{{
		RuleID:   NoRemainingNodesID,
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: "Node", Name: strings.Join(targets, ",")},
		Evidence: findings.NoRemainingNodesEvidence{
			PodsToMove:           moving,
			TargetNodes:          len(targets),
			TotalNodes:           len(nodes),
			RemainingSchedulable: 0,
		},
		Description: fmt.Sprintf(
			"The drain targets %d of %s and no node is left that takes new pods. The %s it evicts are recreated by their controllers and stay Pending until a node can take them.",
			len(targets), plural(len(nodes), "node"), plural(moving, "pod")),
		Remediation: "Drain fewer nodes at once, or uncordon or add a node first, so the evicted pods have somewhere to go. The drain itself will complete; the workloads will be down until then.",
	}}
}
