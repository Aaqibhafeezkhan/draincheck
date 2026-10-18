package findings

// ManagedBy identifies the operator that manages the pods behind a
// PodDisruptionBudget: the API group and kind of the custom resource that
// controls them.
type ManagedBy struct {
	APIGroup string `json:"api_group"`
	Kind     string `json:"kind"`
}

// OperatorManagedPDBsEvidence is the evidence for the PodDisruptionBudgets of
// one operator in one namespace. They are reported together because they share
// one cause: whether the drain stops depends on the operator's state, which
// draincheck cannot see.
type OperatorManagedPDBsEvidence struct {
	ManagedBy ManagedBy `json:"managed_by"`
	Namespace string    `json:"namespace"`
	// PDBs are the budgets, sorted by name. Each has the numbers, nodes and pods
	// that PDB_DISRUPTION_BLOCKED reports for a budget of its own, so grouping
	// hides nothing.
	PDBs []PDBEvidence `json:"pdbs"`
}

func (OperatorManagedPDBsEvidence) isEvidence() {}
