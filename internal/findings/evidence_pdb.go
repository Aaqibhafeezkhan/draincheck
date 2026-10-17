package findings

// PDBPod identifies one pod selected by a PodDisruptionBudget. A budget only
// selects pods of its own namespace, so Namespace always equals the budget's;
// it is kept so that every pod entry in the report has the same shape.
type PDBPod struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// Node is the node the pod is scheduled on.
	Node string `json:"node"`
}

// PDBEvidence is the evidence for PodDisruptionBudget findings. There is one
// finding per budget, however many pods it selects.
type PDBEvidence struct {
	Namespace          string `json:"namespace"`
	Name               string `json:"name"`
	CurrentHealthy     int32  `json:"current_healthy"`
	DesiredHealthy     int32  `json:"desired_healthy"`
	DisruptionsAllowed int32  `json:"disruptions_allowed"`
	// Nodes are the target nodes the selected pods run on, sorted, without
	// duplicates.
	Nodes []string `json:"nodes"`
	// Pods are all pods on the target nodes that the budget selects and
	// kubectl drain would evict, sorted by namespace, name and node.
	Pods []PDBPod `json:"pods"`
}

func (PDBEvidence) isEvidence() {}
