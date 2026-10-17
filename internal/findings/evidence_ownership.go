package findings

// OwnershipPod identifies one pod covered by an ownership finding. Static pods
// of one node can live in different namespaces, so the namespace is kept with
// every pod.
type OwnershipPod struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// Node is the node the pod is scheduled on.
	Node string `json:"node"`
}

// OwnershipEvidence is the evidence for pod ownership findings. One finding
// covers every pod that shares a cause: all pods of a DaemonSet, all static
// pods of a node, or a single pod with no controller.
type OwnershipEvidence struct {
	Category OwnershipCategory `json:"category"`
	// OwnerKind and OwnerName identify the controller; empty for static and
	// unmanaged pods.
	OwnerKind string `json:"owner_kind,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`
	// Nodes are the nodes the covered pods run on, sorted, without duplicates.
	Nodes []string `json:"nodes"`
	// Pods are all pods the finding covers, sorted by namespace, name and node.
	Pods []OwnershipPod `json:"pods"`
}

func (OwnershipEvidence) isEvidence() {}
