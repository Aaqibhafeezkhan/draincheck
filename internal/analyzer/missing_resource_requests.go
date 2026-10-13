package analyzer

import (
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
)

// MissingResourceRequestsID is the rule ID reported by MissingResourceRequests.
const MissingResourceRequestsID = "MISSING_RESOURCE_REQUESTS"

// MissingResourceRequests reports pods on target nodes with containers that
// declare no CPU or memory request. The scheduler treats such a pod as
// needing nothing, so it can land on a node that has no room for it, and
// capacity checks cannot account for it.
//
// One RISK finding is emitted per pod, listing each regular container and
// which request it lacks. Init containers are not inspected: they do not
// contribute to steady-state usage. A request set at pod level covers every
// container for that resource. Pods in a terminal phase (Succeeded, Failed),
// DaemonSet pods and static (mirror) pods are skipped because kubectl drain
// does not evict them.
type MissingResourceRequests struct{}

// ID implements Rule.
func (MissingResourceRequests) ID() string { return MissingResourceRequestsID }

// Evaluate implements Rule.
func (MissingResourceRequests) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	var out []findings.Finding
	owners := inventory.NewOwnerResolver(in.Snapshot)
	for _, pod := range in.Snapshot.Pods() {
		if !slices.Contains(in.Targets, pod.Spec.NodeName) || terminal(pod) ||
			hasAnnotation(pod.Annotations, mirrorPodAnnotation) || owners.Resolve(pod).Kind == "DaemonSet" {
			continue
		}

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
		if len(missing) == 0 {
			continue
		}

		parts := make([]string, len(missing))
		for i, m := range missing {
			var what []string
			if m.MissingCPU {
				what = append(what, "cpu")
			}
			if m.MissingMemory {
				what = append(what, "memory")
			}
			parts[i] = fmt.Sprintf("%s (%s)", m.Name, strings.Join(what, ", "))
		}
		out = append(out, findings.Finding{
			RuleID:   MissingResourceRequestsID,
			Severity: findings.Risk,
			Resource: findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
			Evidence: findings.MissingRequestsEvidence{Node: pod.Spec.NodeName, Containers: missing},
			Description: fmt.Sprintf(
				"Pod %s/%s on node %s has containers without resource requests: %s; the scheduler treats them as needing nothing, so the pod may not fit where it is rescheduled.",
				pod.Namespace, pod.Name, pod.Spec.NodeName, strings.Join(parts, ", ")),
			Remediation: "Set CPU and memory requests on every container (or a pod-level request) so rescheduling and capacity checks can account for the pod.",
		})
	}
	return out
}
