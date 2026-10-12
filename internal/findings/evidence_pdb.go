package findings

// PDBEvidence is the evidence for PodDisruptionBudget findings.
type PDBEvidence struct {
	Namespace          string `json:"namespace"`
	Name               string `json:"name"`
	CurrentHealthy     int32  `json:"current_healthy"`
	DesiredHealthy     int32  `json:"desired_healthy"`
	DisruptionsAllowed int32  `json:"disruptions_allowed"`
}

func (PDBEvidence) isEvidence() {}
