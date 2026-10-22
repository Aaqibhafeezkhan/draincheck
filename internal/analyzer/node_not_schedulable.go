package analyzer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
)

// NodeNotSchedulableID is the rule ID reported by NodeNotSchedulable.
const NodeNotSchedulableID = "NODE_NOT_SCHEDULABLE"

// NodeNotSchedulable reports nodes outside the target set that evicted pods
// cannot land on, because they are cordoned or not Ready (see unschedulable).
// The drain itself completes, but the pods it moves have fewer places to go
// than the node count suggests, and capacity and placement checks computed
// against that node count are overstated.
//
// It is RISK and fires only when the drain moves something: at least one pod
// on a target node that kubectl drain evicts and a controller recreates
// (DaemonSet, static, finished, terminating and controller-less pods need no
// new home, see classifyPod). Without such a pod, a cordoned spare node
// changes nothing about this drain, so it is not reported.
//
// A target node that is already cordoned is not reported: kubectl drain
// cordons the targets itself and says "already cordoned". One finding covers
// every affected node, with each node's state in typed evidence; its resource
// is the affected node names, joined by commas.
type NodeNotSchedulable struct{}

// ID implements Rule.
func (NodeNotSchedulable) ID() string { return NodeNotSchedulableID }

// Evaluate implements Rule.
func (NodeNotSchedulable) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	moving := podsToMove(in.Snapshot, in.Targets)
	if moving == 0 {
		return nil
	}

	remaining := 0
	var blocked []findings.UnschedulableNode
	for _, node := range in.Snapshot.Nodes() {
		if slices.Contains(in.Targets, node.Name) {
			continue
		}
		remaining++
		if state, ok := unschedulable(node); ok {
			blocked = append(blocked, state)
		}
	}
	if len(blocked) == 0 {
		return nil
	}
	slices.SortFunc(blocked, func(a, b findings.UnschedulableNode) int { return strings.Compare(a.Name, b.Name) })

	names := make([]string, len(blocked))
	reasons := make([]string, len(blocked))
	for i, n := range blocked {
		names[i] = n.Name
		reasons[i] = fmt.Sprintf("%s (%s)", n.Name, nodeStateWords(n))
	}
	return []findings.Finding{{
		RuleID:   NodeNotSchedulableID,
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: "Node", Name: strings.Join(names, ",")},
		Evidence: findings.NodeNotSchedulableEvidence{PodsToMove: moving, RemainingNodes: remaining, Nodes: blocked},
		Description: fmt.Sprintf(
			"%d of the %s that stay up cannot take evicted pods: %s. The %s moved off the target nodes can land on at most %s; taints and capacity are not checked.",
			len(blocked), plural(remaining, "node"), strings.Join(reasons, ", "),
			plural(moving, "pod"), plural(remaining-len(blocked), "node")),
		Remediation: "Confirm with `kubectl get nodes` (SchedulingDisabled or NotReady in STATUS) and `kubectl describe node <node>` (Unschedulable, Conditions). Then, safest first: repair the not Ready nodes, or uncordon the cordoned ones once the work they wait for is done; if they must stay out, count only the other nodes as room for the evicted pods. The count in the finding is an upper bound: taints and capacity are not checked.",
	}}
}

// nodeStateWords says why a node takes no pods.
func nodeStateWords(n findings.UnschedulableNode) string {
	notReady := n.ReadyStatus != "" && n.ReadyStatus != "True"
	switch {
	case n.Cordoned && notReady:
		return "cordoned and not Ready"
	case n.Cordoned:
		return "cordoned"
	default:
		return "not Ready"
	}
}
