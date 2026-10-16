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

// MissingResourceRequestsID is the rule ID reported by MissingResourceRequests.
const MissingResourceRequestsID = "MISSING_RESOURCE_REQUESTS"

// MissingResourceRequests reports workloads with pods on target nodes whose
// containers declare no CPU or memory request. The scheduler treats such a pod
// as needing nothing, so it can land on a node that has no room for it, and
// capacity checks cannot account for it.
//
// One RISK finding is emitted per owning workload (Deployment, StatefulSet,
// Job, ...), resolved with inventory.OwnerResolver; a pod with no controller is
// its own workload. The finding points at the workload and carries every
// affected pod, its node and the containers missing a request in typed
// evidence. Init containers are not inspected: they do not contribute to
// steady-state usage. A request set at pod level covers every container for
// that resource. Pods in a terminal phase (Succeeded, Failed), DaemonSet pods
// and static (mirror) pods are skipped because kubectl drain does not evict
// them, so they are never rescheduled.
type MissingResourceRequests struct{}

// ID implements Rule.
func (MissingResourceRequests) ID() string { return MissingResourceRequestsID }

// requestsGroup collects the evicted pods of one workload.
type requestsGroup struct {
	resource findings.ResourceRef
	owner    inventory.Owner
	// total counts every evicted pod of the workload on the target nodes.
	total   int
	missing []findings.MissingRequestsPod
}

// Evaluate implements Rule.
func (MissingResourceRequests) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	owners := inventory.NewOwnerResolver(in.Snapshot)
	byResource := make(map[findings.ResourceRef]*requestsGroup)
	var groups []*requestsGroup // first-seen order, so output does not depend on map order
	for _, pod := range in.Snapshot.Pods() {
		if !slices.Contains(in.Targets, pod.Spec.NodeName) || terminal(pod) ||
			hasAnnotation(pod.Annotations, mirrorPodAnnotation) {
			continue
		}
		owner := owners.Resolve(pod)
		if owner.Kind == "DaemonSet" {
			continue
		}

		ref := findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name}
		if !owner.Unmanaged() {
			ref = findings.ResourceRef{Kind: owner.Kind, Namespace: owner.Namespace, Name: owner.Name}
		}
		g := byResource[ref]
		if g == nil {
			g = &requestsGroup{resource: ref, owner: owner}
			byResource[ref] = g
			groups = append(groups, g)
		}
		g.total++

		if containers := containersMissingRequests(pod); len(containers) > 0 {
			g.missing = append(g.missing, findings.MissingRequestsPod{
				Name: pod.Name, Node: pod.Spec.NodeName, Containers: containers,
			})
		}
	}

	var out []findings.Finding
	for _, g := range groups {
		if len(g.missing) == 0 {
			continue
		}
		slices.SortFunc(g.missing, func(a, b findings.MissingRequestsPod) int {
			return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Node, b.Node))
		})
		out = append(out, findings.Finding{
			RuleID:   MissingResourceRequestsID,
			Severity: findings.Risk,
			Resource: g.resource,
			Evidence: findings.MissingRequestsEvidence{
				OwnerKind:     g.owner.Kind,
				OwnerName:     g.owner.Name,
				PodsOnTargets: g.total,
				Pods:          g.missing,
			},
			Description: describeMissingRequests(g),
			Remediation: "Set CPU and memory requests on every container (or a pod-level request) so rescheduling and capacity checks can account for the workload.",
		})
	}
	return out
}

// containersMissingRequests lists the regular containers of pod that lack a
// CPU or memory request, in spec order. A pod-level request covers every
// container for that resource.
func containersMissingRequests(pod corev1.Pod) []findings.MissingRequestsContainer {
	podCPU, podMemory := false, false
	if r := pod.Spec.Resources; r != nil {
		_, podCPU = r.Requests[corev1.ResourceCPU]
		_, podMemory = r.Requests[corev1.ResourceMemory]
	}

	var missing []findings.MissingRequestsContainer
	for _, c := range pod.Spec.Containers {
		_, hasCPU := c.Resources.Requests[corev1.ResourceCPU]
		_, hasMemory := c.Resources.Requests[corev1.ResourceMemory]
		m := findings.MissingRequestsContainer{
			Name:          c.Name,
			MissingCPU:    !hasCPU && !podCPU,
			MissingMemory: !hasMemory && !podMemory,
		}
		if m.MissingCPU || m.MissingMemory {
			missing = append(missing, m)
		}
	}
	return missing
}

// describeMissingRequests states the count once and names each distinct
// container shortfall once, however many replicas share it. The per-pod detail
// is in the evidence.
func describeMissingRequests(g *requestsGroup) string {
	var shortfalls []string
	for _, pod := range g.missing {
		for _, m := range pod.Containers {
			var what []string
			if m.MissingCPU {
				what = append(what, "cpu")
			}
			if m.MissingMemory {
				what = append(what, "memory")
			}
			s := fmt.Sprintf("%s (%s)", m.Name, strings.Join(what, ", "))
			if !slices.Contains(shortfalls, s) {
				shortfalls = append(shortfalls, s)
			}
		}
	}

	var subject string
	if g.owner.Unmanaged() {
		subject = fmt.Sprintf("pod %s/%s on node %s", g.resource.Namespace, g.resource.Name, g.missing[0].Node)
	} else {
		noun := "pods"
		if g.total == 1 {
			noun = "pod"
		}
		subject = fmt.Sprintf("%d of %d %s of %s %s/%s on the target nodes",
			len(g.missing), g.total, noun, g.owner.Kind, g.owner.Namespace, g.owner.Name)
	}
	return fmt.Sprintf(
		"Missing resource requests in %s: %s. The scheduler treats a missing request as needing nothing, so rescheduled pods may not fit where they land.",
		subject, strings.Join(shortfalls, ", "))
}
