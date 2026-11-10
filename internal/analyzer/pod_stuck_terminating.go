package analyzer

import (
	"fmt"
	"slices"
	"time"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
)

// PodStuckTerminatingID is the rule ID reported by PodStuckTerminating.
const PodStuckTerminatingID = "POD_STUCK_TERMINATING"

// stuckTerminatingMargin is how long after its deadline a terminating pod must
// still be there before it counts as stuck: the kubelet needs a moment to
// report the pod gone, and the snapshot is read a moment after the cluster.
const stuckTerminatingMargin = time.Minute

// PodStuckTerminating reports a pod on a target node that is being deleted and
// is still there more than stuckTerminatingMargin after its deadline. The
// deadline is the pod's deletionTimestamp, which already includes the grace
// period, so a pod still inside its grace period is never reported. kubectl
// drain waits for such a pod to disappear. It is RISK: nothing refuses it.
//
// The time is the snapshot's, never the clock, so one snapshot gives one answer.
// A finished pod counts: drain deletes it and waits for it like any other, and
// a finalizer keeps it. DaemonSet and static pods (skipped by drain) and pods
// --pod-selector leaves out do not. One finding per pod.
type PodStuckTerminating struct{}

// ID implements Rule.
func (PodStuckTerminating) ID() string { return PodStuckTerminatingID }

// Evaluate implements Rule.
func (PodStuckTerminating) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}
	now := in.Snapshot.Timestamp()
	owners := inventory.NewOwnerResolver(in.Snapshot)

	var out []findings.Finding
	for _, pod := range in.Snapshot.Pods() {
		if !slices.Contains(in.Targets, pod.Spec.NodeName) {
			continue
		}
		d := classifyPod(pod, owners, in.Drain)
		deadline, ok := terminatingDeadline(pod)
		if !ok || !d.waitsForDeletion() || !now.After(deadline.Add(stuckTerminatingMargin)) {
			continue
		}

		finalizers := slices.Clone(pod.Finalizers)
		if finalizers == nil {
			finalizers = []string{}
		}
		c := podStuckTerminatingNoFinalizer
		if len(finalizers) > 0 {
			c = podStuckTerminatingFinalizer
		}
		out = append(out, findings.Finding{
			RuleID:   PodStuckTerminatingID,
			Severity: findings.Risk,
			Resource: findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
			Evidence: findings.StuckTerminatingEvidence{
				Node:               pod.Spec.NodeName,
				Deadline:           deadline,
				GracePeriodSeconds: terminationGrace(pod),
				Finalizers:         finalizers,
			},
			Case: c,
			Description: fmt.Sprintf(
				"Pod %s/%s on node %s is still terminating %s after its deadline (grace period %ds, %d finalizers).",
				pod.Namespace, pod.Name, pod.Spec.NodeName, now.Sub(deadline).Round(time.Second), terminationGrace(pod), len(finalizers)),
		})
	}
	return out
}
