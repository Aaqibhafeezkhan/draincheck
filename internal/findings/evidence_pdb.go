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
	// Pods are the pods on the target nodes that the budget selects and
	// kubectl drain would evict, and that eviction puts through the budget,
	// sorted by namespace, name and node.
	Pods []PDBPod `json:"pods"`
	// UnhealthyPodEvictionPolicy is spec.unhealthyPodEvictionPolicy as
	// configured; empty when unset, which means IfHealthyBudget.
	UnhealthyPodEvictionPolicy string `json:"unhealthy_pod_eviction_policy,omitempty"`
	// EvictableUnhealthyPods are pods the budget selects that are not Ready and
	// that the eviction API lets through whatever the budget says (see the
	// policy). They are not in Pods and do not use up the budget. Sorted like
	// Pods; omitted when there are none.
	EvictableUnhealthyPods []PDBPod `json:"evictable_unhealthy_pods,omitempty"`
}

func (PDBEvidence) isEvidence() {}
