package findings

// PDBSyncFailedEvidence is the evidence for a PodDisruptionBudget whose
// disruption controller could not compute the budget. It carries everything
// PDBEvidence does, with the numbers as last reported, and the failure the
// controller recorded.
type PDBSyncFailedEvidence struct {
	PDBEvidence
	// Reason is the reason of the DisruptionAllowed condition (SyncFailed).
	Reason string `json:"reason"`
	// Message is the message of the DisruptionAllowed condition: the error the
	// controller hit. It may be empty.
	Message string `json:"message"`
}

func (PDBSyncFailedEvidence) isEvidence() {}
