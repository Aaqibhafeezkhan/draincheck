package findings

// OverlappingPDB is one of the budgets that select a pod, with the numbers its
// status reports. They are kept as reported and are not to be trusted: the
// disruption controller chooses one of the budgets that select a pod when the
// pod changes, and the others were seen to keep the numbers of an earlier sync.
type OverlappingPDB struct {
	Name               string `json:"name"`
	CurrentHealthy     int32  `json:"current_healthy"`
	DesiredHealthy     int32  `json:"desired_healthy"`
	DisruptionsAllowed int32  `json:"disruptions_allowed"`
}

// OverlappingPDBsEvidence is the evidence for pods selected by more than one
// PodDisruptionBudget. PDBs are always in the same namespace as the pod and
// are sorted by name. Budgets holds the same budgets in the same order, each
// with the numbers its status reports.
type OverlappingPDBsEvidence struct {
	Namespace string           `json:"namespace"`
	PDBs      []string         `json:"pdbs"`
	Budgets   []OverlappingPDB `json:"budgets"`
}

func (OverlappingPDBsEvidence) isEvidence() {}
