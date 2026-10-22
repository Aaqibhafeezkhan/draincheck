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
// checks. A pod that is already terminating is still reported: kubectl drain
// applies these checks to it as well.
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
		if !slices.Contains(in.Targets, pod.Spec.NodeName) {
			continue
		}

		// Ownership is judged by class, not by fate: kubectl drain applies its
		// DaemonSet, static and unreplicated checks to a pod that is already
		// terminating too.
		var ref findings.ResourceRef
		var category findings.OwnershipCategory
		owner := owners.Resolve(pod)
		switch classifyPod(pod, owners).class {
		case classStatic:
			category = findings.OwnershipStaticPod
			ref = findings.ResourceRef{Kind: "Node", Name: pod.Spec.NodeName}
		case classUnmanaged:
			category = findings.OwnershipUnmanaged
			ref = findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name}
		case classDaemonSet:
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
			"Confirm with `kubectl get pods -A --field-selector spec.nodeName=<node>` and `kubectl get pod -n <namespace> <pod> -o jsonpath='{.metadata.annotations}'` (the annotation kubernetes.io/config.mirror marks a static pod). Then plan for the static pods to be unavailable while the node is down; kubectl drain skips them."
	case findings.OwnershipDaemonSet:
		return fmt.Sprintf(
				"DaemonSet %s/%s has %s on %s. kubectl drain refuses DaemonSet pods without --ignore-daemonsets and never evicts them.",
				g.resource.Namespace, g.resource.Name, plural(len(g.pods), "pod"), plural(nodeCount, "target node")),
			"Confirm with `kubectl get daemonset -n <namespace> <name>` and `kubectl get pods -A --field-selector spec.nodeName=<node> -o wide`. Then drain with --ignore-daemonsets; DaemonSet pods stay until the node itself goes down. If the DaemonSet is not found, its pods are orphans, which kubectl drain treats differently."
	default:
		pod := g.pods[0]
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
