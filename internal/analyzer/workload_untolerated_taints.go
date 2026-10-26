package analyzer

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
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
// listing the nodes left and the taints the workload does not tolerate. Like
// NoRemainingNodes it judges cordons, readiness and taints only, never
// affinity, selectors or capacity.
type WorkloadUntoleratedTaints struct{}

// ID implements Rule.
func (WorkloadUntoleratedTaints) ID() string { return WorkloadUntoleratedTaintsID }

// untoleratedGroup collects the moving pods of one workload.
type untoleratedGroup struct {
	owner inventory.Owner
	pods  []corev1.Pod
}

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

	owners := inventory.NewOwnerResolver(in.Snapshot)
	byOwner := map[inventory.Owner]*untoleratedGroup{}
	var groups []*untoleratedGroup
	for _, pod := range moving {
		owner := owners.Resolve(pod)
		g := byOwner[owner]
		if g == nil {
			g = &untoleratedGroup{owner: owner}
			byOwner[owner] = g
			groups = append(groups, g)
		}
		g.pods = append(g.pods, pod)
	}

	var stranded []*untoleratedGroup
	for _, g := range groups {
		if !hasRoom(g.pods, nodes) {
			stranded = append(stranded, g)
		}
	}
	// Every workload stranded: the run has no room, which NoRemainingNodes states.
	if len(stranded) == 0 || len(stranded) == len(groups) {
		return nil
	}

	slices.SortFunc(stranded, func(a, b *untoleratedGroup) int {
		return cmp.Or(cmp.Compare(a.owner.Namespace, b.owner.Namespace), cmp.Compare(a.owner.Kind, b.owner.Kind), cmp.Compare(a.owner.Name, b.owner.Name))
	})
	out := make([]findings.Finding, 0, len(stranded))
	for _, g := range stranded {
		ev := untoleratedEvidence(g, nodes)
		out = append(out, findings.Finding{
			RuleID:   WorkloadUntoleratedTaintsID,
			Severity: findings.Risk,
			Resource: findings.ResourceRef{Kind: g.owner.Kind, Namespace: g.owner.Namespace, Name: g.owner.Name},
			Evidence: ev,
			Description: fmt.Sprintf(
				"%s %s/%s has %s on the target nodes and none can land on the %s left: %s. Other pods of this drain can. The controller recreates the pods and they stay Pending until a node takes them. kubectl drain only evicts, so it completes.",
				g.owner.Kind, g.owner.Namespace, g.owner.Name, plural(len(ev.Pods), "pod"), plural(len(nodes), "node"), taintList(ev.TaintedNodes)),
			Remediation: "Confirm with `kubectl describe node <node>` for a node named in the finding (the taints are under Taints) and `kubectl get pod -n <namespace> <pod> -o jsonpath='{.spec.tolerations}'` for a pod of the workload. Then, safest first: add or uncordon a node the workload tolerates before draining; or drain fewer nodes at once; or give the workload a toleration for the taint, only if that node is meant to run it. The drain itself completes either way; afterwards `kubectl get pods -A --field-selector=status.phase=Pending` shows what is waiting. Cordons, readiness and taints are checked, not affinity or capacity.",
		})
	}
	return out
}

// hasRoom reports whether any of the pods can land on any of the nodes as far
// as taints go.
func hasRoom(pods []corev1.Pod, nodes []corev1.Node) bool {
	for _, node := range nodes {
		if _, room := taintsAgainst(pods, node); room {
			return true
		}
	}
	return false
}

// untoleratedEvidence builds the evidence for a workload that hasRoom reported false for.
func untoleratedEvidence(g *untoleratedGroup, nodes []corev1.Node) findings.WorkloadUntoleratedTaintsEvidence {
	ev := findings.WorkloadUntoleratedTaintsEvidence{OwnerKind: g.owner.Kind, OwnerName: g.owner.Name, RemainingNodes: len(nodes)}
	for _, pod := range g.pods {
		ev.Pods = append(ev.Pods, findings.StrandedPod{Name: pod.Name, Node: pod.Spec.NodeName})
	}
	slices.SortFunc(ev.Pods, func(a, b findings.StrandedPod) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Node, b.Node))
	})
	for _, node := range nodes {
		taints, _ := taintsAgainst(g.pods, node)
		ev.TaintedNodes = append(ev.TaintedNodes, findings.TaintedNode{Name: node.Name, Taints: taints})
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
