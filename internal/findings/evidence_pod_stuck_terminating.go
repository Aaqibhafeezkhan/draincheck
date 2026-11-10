package findings

import "time"

// StuckTerminatingEvidence is the evidence for a pod on a target node that is
// still terminating well after its deadline.
type StuckTerminatingEvidence struct {
	// Node is the target node the pod runs on.
	Node string `json:"node"`
	// Deadline is the pod's metadata.deletionTimestamp, the time it should be gone:
	// that is the time deletion started plus the grace period.
	Deadline time.Time `json:"deadline"`
	// GracePeriodSeconds is the pod's terminationGracePeriodSeconds (30 when unset).
	GracePeriodSeconds int64 `json:"grace_period_seconds"`
	// Finalizers are the pod's metadata.finalizers, in the order the pod lists
	// them; empty (not null) when there are none.
	Finalizers []string `json:"finalizers"`
}

func (StuckTerminatingEvidence) isEvidence() {}
