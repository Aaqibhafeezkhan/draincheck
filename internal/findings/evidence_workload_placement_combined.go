package findings

// RuledOutNode is a node that stays up and is Ready, but that none of the pods
// of a workload can land on, with every reason: the taints and the label
// requirements together. Either list may be empty, never both.
type RuledOutNode struct {
	Name string `json:"name"`
	// Taints are the NoSchedule and NoExecute taints that at least one pod of
	// the workload does not tolerate, in the order the node lists them.
	Taints []NodeTaint `json:"taints"`
	// Mismatches are the requirements that at least one pod of the workload
	// does not meet on this node, sorted by source, key, operator and values.
	Mismatches []LabelMismatch `json:"mismatches"`
}

// WorkloadPlacementCombinedEvidence is the evidence for a workload none of whose
// evicted pods can land on a node left, where neither taints alone nor labels
// alone explain it: taints rule out some nodes and labels the others, or each
// rules out every node. There is one per workload. The pods are in the
// namespace of the finding's resource.
type WorkloadPlacementCombinedEvidence struct {
	// OwnerKind and OwnerName identify the controller (Deployment,
	// StatefulSet, ...).
	OwnerKind string `json:"owner_kind"`
	OwnerName string `json:"owner_name"`
	// Pods are the evicted pods of the workload, sorted by name then node.
	Pods []StrandedPod `json:"pods"`
	// RemainingNodes is the number of nodes outside the target set that take
	// new pods: not cordoned and not NotReady.
	RemainingNodes int `json:"remaining_nodes"`
	// RuledOutNodes are those remaining nodes, sorted by name, each with its
	// reasons. Every remaining node is listed, because a workload is reported
	// only when each of them rules it out.
	RuledOutNodes []RuledOutNode `json:"ruled_out_nodes"`
}

func (WorkloadPlacementCombinedEvidence) isEvidence() {}
