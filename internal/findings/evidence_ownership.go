package findings

// OwnershipEvidence is the evidence for pod ownership findings.
type OwnershipEvidence struct {
	Category OwnershipCategory `json:"category"`
	// Node is the node the pod is scheduled on.
	Node string `json:"node"`
	// OwnerKind and OwnerName identify the controller; empty for static and
	// unmanaged pods.
	OwnerKind string `json:"owner_kind,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`
}

func (OwnershipEvidence) isEvidence() {}
