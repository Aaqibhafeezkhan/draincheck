package analyzer

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
)

// PodOwnershipID is the rule ID reported by PodOwnership.
const PodOwnershipID = "POD_OWNERSHIP"

// PodOwnership reports pods on target nodes whose ownership gives them special
// drain behaviour: DaemonSet-managed pods, static (mirror) pods and naked pods
// with no controller.
//
// One finding is emitted per cause, not per pod:
//   - per DaemonSet, covering all of its pods on the target nodes;
//   - per node, covering the static pods that stop when that node goes down;
//   - per pod, for a pod with no controller, because each one is lost on drain.
//
// Every pod and node stays in the typed evidence. The severity follows what
// kubectl drain does with the pod under the flags the operator passes
// (Input.Drain):
//   - a DaemonSet pod is BLOCKED without --ignore-daemonsets, because kubectl
//     drain refuses it; with the flag kubectl skips it and nothing is reported;
//   - a pod with no controller is BLOCKED without --force, for the same reason;
//     with the flag kubectl deletes it and nothing recreates it, so RISK;
//   - a static pod is skipped whatever the flags are: RISK, to plan for the
//     outage of the node.
//
// Pods managed by any other controller are not reported. Pods in a terminal
// phase (Succeeded, Failed) are skipped because kubectl drain removes them
// without further checks. A pod that is already terminating is still reported:
// kubectl drain applies these checks to it as well. Only the ownership filters
// are asked; the emptyDir filter belongs to a rule of its own (#29).
type PodOwnership struct{}

// ID implements Rule.
func (PodOwnership) ID() string { return PodOwnershipID }

// ownershipGroup collects the pods that share one cause.
type ownershipGroup struct {
	resource findings.ResourceRef
	category findings.OwnershipCategory
	severity findings.Severity
	pods     []findings.OwnershipPod
}

// Evaluate implements Rule.
func (PodOwnership) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	owners := inventory.NewOwnerResolver(in.Snapshot)
	byResource := make(map[findings.ResourceRef]*ownershipGroup)
	var groups []*ownershipGroup // first-seen order, so output does not depend on map order
	for _, pod := range in.Snapshot.Pods() {
		if !slices.Contains(in.Targets, pod.Spec.NodeName) {
			continue
		}

		// Ownership is judged by class, not by fate: kubectl drain applies its
		// DaemonSet, static and unreplicated checks to a pod that is already
		// terminating too.
		var ref findings.ResourceRef
		var category findings.OwnershipCategory
		var severity findings.Severity
		owner := owners.Resolve(pod)
		drain := classifyPod(pod, owners, in.Drain)
		if !drain.selected {
			continue
		}
		switch drain.class {
		case classStatic:
			category, severity = findings.OwnershipStaticPod, findings.Risk
			ref = findings.ResourceRef{Kind: "Node", Name: pod.Spec.NodeName}
		case classUnmanaged:
			category = findings.OwnershipUnmanaged
			ref = findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name}
			severity = unmanagedSeverity(unreplicatedFilter(pod, in.Drain))
		case classDaemonSet:
			category = findings.OwnershipDaemonSet
			severity = daemonSetSeverity(daemonSetFilter(pod, in.Drain))
			ref = findings.ResourceRef{Kind: owner.Kind, Namespace: owner.Namespace, Name: owner.Name}
		default:
			continue
		}
		if severity == "" {
			continue // kubectl drain lets the pod pass or skips it: nothing to report
		}

		g := byResource[ref]
		if g == nil {
			g = &ownershipGroup{resource: ref, category: category, severity: severity}
			byResource[ref] = g
			groups = append(groups, g)
		}
		g.pods = append(g.pods, findings.OwnershipPod{Namespace: pod.Namespace, Name: pod.Name, Node: pod.Spec.NodeName})
	}

	var out []findings.Finding
	for _, g := range groups {
		slices.SortFunc(g.pods, func(a, b findings.OwnershipPod) int {
			return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name), cmp.Compare(a.Node, b.Node))
		})
		nodes := make([]string, len(g.pods))
		for i, p := range g.pods {
			nodes[i] = p.Node
		}
		slices.Sort(nodes)
		nodes = slices.Compact(nodes)

		ev := findings.OwnershipEvidence{Category: g.category, Nodes: nodes, Pods: g.pods}
		if g.category == findings.OwnershipDaemonSet {
			ev.OwnerKind, ev.OwnerName = g.resource.Kind, g.resource.Name
		}
		description, remediation := describeOwnership(g, len(nodes))
		out = append(out, findings.Finding{
			RuleID:      PodOwnershipID,
			Severity:    g.severity,
			Resource:    g.resource,
			Evidence:    ev,
			Description: description,
			Remediation: remediation,
		})
	}
	return out
}

// daemonSetSeverity maps the answer of kubectl's DaemonSet filter to a
// severity: BLOCKED when it refuses the pod, none when it skips the pod or does
// not see a DaemonSet pod (a controller of kind DaemonSet outside apps/v1).
func daemonSetSeverity(v drainVerdict) findings.Severity {
	if v.action == drainRefuse {
		return findings.Blocked
	}
	return ""
}

// unmanagedSeverity maps the answer of kubectl's unreplicated filter to a
// severity: BLOCKED when it refuses the pod, RISK when --force lets kubectl
// delete a pod that nothing recreates.
func unmanagedSeverity(v drainVerdict) findings.Severity {
	switch v.action {
	case drainRefuse:
		return findings.Blocked
	case drainDelete:
		return findings.Risk
	}
	return ""
}

// describeOwnership returns the description and remediation for g, which
// covers pods on nodeCount distinct nodes. The pods themselves are in the
// evidence; static pods are also named here because they are the whole point
// of their finding. The text says what kubectl does under the flags in force:
// a refusal for BLOCKED, a deletion for the RISK of a pod with no controller.
func describeOwnership(g *ownershipGroup, nodeCount int) (description, remediation string) {
	switch g.category {
	case findings.OwnershipStaticPod:
		names := make([]string, len(g.pods))
		for i, p := range g.pods {
			names[i] = p.Namespace + "/" + p.Name
		}
		return fmt.Sprintf(
				"Node %s runs %s (%s). Static pods cannot be evicted or rescheduled and stop when the node goes down.",
				g.resource.Name, plural(len(g.pods), "static pod"), strings.Join(names, ", ")),
			"Confirm with `kubectl get pods -A --field-selector spec.nodeName=<node>` and `kubectl get pod -n <namespace> <pod> -o jsonpath='{.metadata.annotations}'` (the annotation kubernetes.io/config.mirror marks a static pod). Then plan for the static pods to be unavailable while the node is down; kubectl drain skips them."
	case findings.OwnershipDaemonSet:
		return fmt.Sprintf(
				"DaemonSet %s/%s has %s on %s. kubectl drain refuses DaemonSet pods without --ignore-daemonsets and never evicts them.",
				g.resource.Namespace, g.resource.Name, plural(len(g.pods), "pod"), plural(nodeCount, "target node")),
			"Confirm with `kubectl get daemonset -n <namespace> <name>` and `kubectl get pods -A --field-selector spec.nodeName=<node> -o wide`. Then drain with --ignore-daemonsets; DaemonSet pods stay until the node itself goes down. If the DaemonSet is not found, its pods are orphans, which kubectl drain treats differently."
	default:
		pod := g.pods[0]
		if g.severity == findings.Risk {
			return fmt.Sprintf(
					"Pod %s/%s on node %s has no controller. With --force kubectl drain deletes it and it will not be recreated.",
					pod.Namespace, pod.Name, pod.Node),
				"Confirm with `kubectl get pod -n <namespace> <name> -o jsonpath='{.metadata.ownerReferences}'` (empty: no controller). Then, safest first: put the workload under a controller (Deployment, StatefulSet or Job) so it is recreated; or accept its loss."
		}
		return fmt.Sprintf(
				"Pod %s/%s on node %s has no controller; kubectl drain refuses it without --force and it will not be recreated.",
				pod.Namespace, pod.Name, pod.Node),
			"Confirm with `kubectl get pod -n <namespace> <name> -o jsonpath='{.metadata.ownerReferences}'` (empty: no controller). Then, safest first: put the workload under a controller (Deployment, StatefulSet or Job) so it is recreated; or accept its loss and drain with --force."
	}
}

// plural renders "1 pod" or "3 pods".
func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func hasAnnotation(annotations map[string]string, key string) bool {
	_, ok := annotations[key]
	return ok
}
