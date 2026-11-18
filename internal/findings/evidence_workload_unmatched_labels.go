package findings

// WorkloadUnmatchedLabelsEvidence is the evidence for a workload none of whose
// evicted pods can land on a node left, because the labels of every node left
// rule them out, while taints leave at least one of those nodes open. There is
// one per workload. The pods are in the namespace of the finding's resource.
type WorkloadUnmatchedLabelsEvidence struct {
	// OwnerKind and OwnerName identify the controller (Deployment,
	// StatefulSet, ...).
	OwnerKind string `json:"owner_kind"`
	OwnerName string `json:"owner_name"`
	// Pods are the evicted pods of the workload, sorted by name then node.
	Pods []StrandedPod `json:"pods"`
	// RemainingNodes is the number of nodes outside the target set that take
	// new pods: not cordoned and not NotReady.
	RemainingNodes int `json:"remaining_nodes"`
	// MismatchedNodes are those remaining nodes, sorted by name, with the
	// requirements the workload's pods do not meet. Every remaining node is
	// listed, because a workload is reported only when each of them rules it
	// out.
	MismatchedNodes []MismatchedNode `json:"mismatched_nodes"`
}

func (WorkloadUnmatchedLabelsEvidence) isEvidence() {}
