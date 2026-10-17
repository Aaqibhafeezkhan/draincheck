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

// mirrorPodAnnotation is set by the kubelet on the API object that mirrors a
// static pod.
const mirrorPodAnnotation = "kubernetes.io/config.mirror"

// PodOwnership reports pods on target nodes whose ownership gives them special
// drain behaviour: DaemonSet-managed pods, static (mirror) pods and naked pods
// with no controller.
//
// One RISK finding is emitted per cause, not per pod:
//   - per DaemonSet, covering all of its pods on the target nodes;
//   - per node, covering the static pods that stop when that node goes down;
//   - per pod, for a pod with no controller, because each one is lost on drain.
//
// Every pod and node stays in the typed evidence. They are not BLOCKED because
// whether a drain stops depends on kubectl flags the analyzer cannot observe,
// and none of them is a violated constraint on its own. Pods managed by any
// other controller are not reported. Pods in a terminal phase (Succeeded,
// Failed) are skipped because kubectl drain removes them without further
// checks.
type PodOwnership struct{}

// ID implements Rule.
func (PodOwnership) ID() string { return PodOwnershipID }

// ownershipGroup collects the pods that share one cause.
type ownershipGroup struct {
	resource findings.ResourceRef
	category findings.OwnershipCategory
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
		if !slices.Contains(in.Targets, pod.Spec.NodeName) || terminal(pod) {
			continue
		}

		var ref findings.ResourceRef
		var category findings.OwnershipCategory
		owner := owners.Resolve(pod)
		switch {
		// A mirror pod also carries a Node owner reference, so it is checked first.
		case hasAnnotation(pod.Annotations, mirrorPodAnnotation):
			category = findings.OwnershipStaticPod
			ref = findings.ResourceRef{Kind: "Node", Name: pod.Spec.NodeName}
		case owner.Unmanaged():
			category = findings.OwnershipUnmanaged
			ref = findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name}
		case owner.Kind == "DaemonSet":
			category = findings.OwnershipDaemonSet
			ref = findings.ResourceRef{Kind: owner.Kind, Namespace: owner.Namespace, Name: owner.Name}
		default:
			continue
		}

		g := byResource[ref]
		if g == nil {
			g = &ownershipGroup{resource: ref, category: category}
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
			Severity:    findings.Risk,
			Resource:    g.resource,
			Evidence:    ev,
			Description: description,
			Remediation: remediation,
		})
	}
	return out
}

// describeOwnership returns the description and remediation for g, which
// covers pods on nodeCount distinct nodes. The pods themselves are in the
// evidence; static pods are also named here because they are the whole point
// of their finding.
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
			"Plan for the static pods to be unavailable while the node is down; kubectl drain skips them."
	case findings.OwnershipDaemonSet:
		return fmt.Sprintf(
				"DaemonSet %s/%s has %s on %s. kubectl drain refuses DaemonSet pods without --ignore-daemonsets and never evicts them.",
				g.resource.Namespace, g.resource.Name, plural(len(g.pods), "pod"), plural(nodeCount, "target node")),
			"Drain with --ignore-daemonsets; DaemonSet pods stay until the node itself goes down."
	default:
		pod := g.pods[0]
		return fmt.Sprintf(
				"Pod %s/%s on node %s has no controller; kubectl drain refuses it without --force and it will not be recreated.",
				pod.Namespace, pod.Name, pod.Node),
			"Put the pod under a controller, or accept its loss and drain with --force."
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
