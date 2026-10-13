package analyzer

import (
	"fmt"
	"slices"

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
// One RISK finding is emitted per pod. They are not BLOCKED because whether a
// drain stops depends on kubectl flags the analyzer cannot observe, and none of
// them is a violated constraint on its own. Pods managed by any other
// controller are not reported. Pods in a terminal phase (Succeeded, Failed) are
// skipped because kubectl drain removes them without further checks.
type PodOwnership struct{}

// ID implements Rule.
func (PodOwnership) ID() string { return PodOwnershipID }

// Evaluate implements Rule.
func (PodOwnership) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	var out []findings.Finding
	owners := inventory.NewOwnerResolver(in.Snapshot)
	for _, pod := range in.Snapshot.Pods() {
		if !slices.Contains(in.Targets, pod.Spec.NodeName) || terminal(pod) {
			continue
		}

		ev := findings.OwnershipEvidence{Node: pod.Spec.NodeName}
		var description, remediation string
		owner := owners.Resolve(pod)
		switch {
		// A mirror pod also carries a Node owner reference, so it is checked first.
		case hasAnnotation(pod.Annotations, mirrorPodAnnotation):
			ev.Category = findings.OwnershipStaticPod
			description = fmt.Sprintf(
				"Pod %s/%s is a static pod mirrored from node %s; it cannot be evicted or rescheduled and stops with the node.",
				pod.Namespace, pod.Name, pod.Spec.NodeName)
			remediation = "Plan for the static pod to be unavailable while the node is down; kubectl drain skips it."
		case owner.Unmanaged():
			ev.Category = findings.OwnershipUnmanaged
			description = fmt.Sprintf(
				"Pod %s/%s on node %s has no controller; kubectl drain refuses it without --force and it will not be recreated.",
				pod.Namespace, pod.Name, pod.Spec.NodeName)
			remediation = "Put the pod under a controller, or accept its loss and drain with --force."
		case owner.Kind == "DaemonSet":
			ev.Category = findings.OwnershipDaemonSet
			ev.OwnerKind, ev.OwnerName = owner.Kind, owner.Name
			description = fmt.Sprintf(
				"Pod %s/%s on node %s is managed by DaemonSet %s; kubectl drain refuses it without --ignore-daemonsets and never evicts it.",
				pod.Namespace, pod.Name, pod.Spec.NodeName, owner.Name)
			remediation = "Drain with --ignore-daemonsets; the DaemonSet pod stays until the node itself goes down."
		default:
			continue
		}

		out = append(out, findings.Finding{
			RuleID:      PodOwnershipID,
			Severity:    findings.Risk,
			Resource:    findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
			Evidence:    ev,
			Description: description,
			Remediation: remediation,
		})
	}
	return out
}

func hasAnnotation(annotations map[string]string, key string) bool {
	_, ok := annotations[key]
	return ok
}
