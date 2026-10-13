package findings

// OverlappingPDBsEvidence is the evidence for pods selected by more than one
// PodDisruptionBudget. PDBs are always in the same namespace as the pod and
// are sorted by name.
type OverlappingPDBsEvidence struct {
	Namespace string   `json:"namespace"`
	PDBs      []string `json:"pdbs"`
}

func (OverlappingPDBsEvidence) isEvidence() {}
