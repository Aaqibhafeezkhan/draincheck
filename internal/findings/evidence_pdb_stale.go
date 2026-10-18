package findings

// PDBStaleStatusEvidence is the evidence for a PodDisruptionBudget whose status
// has not caught up with its spec. It carries what a PDB_DISRUPTION_BLOCKED
// finding would have shown, with the numbers as last reported (they cannot be
// trusted), so replacing that finding hides nothing.
type PDBStaleStatusEvidence struct {
	PDBEvidence
	// ObservedGeneration is status.observedGeneration: the spec generation the
	// disruption controller last reconciled.
	ObservedGeneration int64 `json:"observed_generation"`
	// Generation is metadata.generation: the current spec generation.
	Generation int64 `json:"generation"`
}

func (PDBStaleStatusEvidence) isEvidence() {}
