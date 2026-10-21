package findings

// NoRemainingNodesEvidence is the evidence for a drain that leaves no node able
// to take the evicted pods.
type NoRemainingNodesEvidence struct {
	// PodsToMove is the number of pods on the target nodes that a drain evicts
	// and that a controller recreates elsewhere.
	PodsToMove int `json:"pods_to_move"`
	// TargetNodes is the number of nodes being drained.
	TargetNodes int `json:"target_nodes"`
	// TotalNodes is the number of nodes in the snapshot.
	TotalNodes int `json:"total_nodes"`
	// RemainingSchedulable is the number of nodes outside the target set that
	// take new pods: not cordoned and not NotReady. It is 0 whenever this
	// evidence is reported; it is kept so the report states the number the
	// finding is about.
	RemainingSchedulable int `json:"remaining_schedulable"`
}

func (NoRemainingNodesEvidence) isEvidence() {}
