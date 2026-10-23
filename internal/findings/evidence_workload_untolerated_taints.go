package findings

// StrandedPod is a pod of the workload that the drain evicts and that cannot
// land on any node left.
type StrandedPod struct {
	Name string `json:"name"`
	// Node is the target node the pod runs on now.
	Node string `json:"node"`
}

// WorkloadUntoleratedTaintsEvidence is the evidence for a workload none of whose evicted
// pods can land on a node left, while pods of other workloads can. There is one
// per workload. The pods are in the namespace of the finding's resource.
type WorkloadUntoleratedTaintsEvidence struct {
	// OwnerKind and OwnerName identify the controller (Deployment,
	// StatefulSet, ...).
	OwnerKind string `json:"owner_kind"`
	OwnerName string `json:"owner_name"`
	// Pods are the evicted pods of the workload, sorted by name then node.
	Pods []StrandedPod `json:"pods"`
	// RemainingNodes is the number of nodes outside the target set that take
	// new pods: not cordoned and not NotReady.
	RemainingNodes int `json:"remaining_nodes"`
	// TaintedNodes are those remaining nodes, sorted by name, with the taints
	// the workload does not tolerate. Every remaining node is listed, because a
	// workload is reported only when each of them rules it out.
	TaintedNodes []TaintedNode `json:"tainted_nodes"`
}

func (WorkloadUntoleratedTaintsEvidence) isEvidence() {}
