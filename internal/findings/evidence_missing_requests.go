package findings

// MissingRequestsContainer names a container and which of its CPU and memory
// requests are absent.
type MissingRequestsContainer struct {
	Name          string `json:"name"`
	MissingCPU    bool   `json:"missing_cpu"`
	MissingMemory bool   `json:"missing_memory"`
}

// MissingRequestsEvidence is the evidence for pods whose containers lack CPU
// or memory requests. Containers are listed in pod spec order and only when at
// least one request is missing.
type MissingRequestsEvidence struct {
	// Node is the node the pod is scheduled on.
	Node       string                     `json:"node"`
	Containers []MissingRequestsContainer `json:"containers"`
}

func (MissingRequestsEvidence) isEvidence() {}
