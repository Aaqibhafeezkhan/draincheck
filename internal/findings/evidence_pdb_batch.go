package findings

// PDBBatchEvidence is the evidence for a PodDisruptionBudget that the whole
// batch of target nodes would disrupt more than it allows at once. It carries
// everything PDBEvidence does, with the numbers as reported, and the number of
// pods the batch evicts.
type PDBBatchEvidence struct {
	PDBEvidence
	// PodsToEvict is the number of pods on all target nodes that the budget
	// selects and kubectl drain would evict. It is greater than
	// DisruptionsAllowed. It ignores unhealthyPodEvictionPolicy.
	PodsToEvict int `json:"pods_to_evict"`
}

func (PDBBatchEvidence) isEvidence() {}
