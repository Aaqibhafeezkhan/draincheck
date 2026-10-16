package findings

// MissingRequestsContainer names a container and which of its CPU and memory
// requests are absent.
type MissingRequestsContainer struct {
	Name          string `json:"name"`
	MissingCPU    bool   `json:"missing_cpu"`
	MissingMemory bool   `json:"missing_memory"`
}

// MissingRequestsPod is one pod of a workload that has containers without CPU
// or memory requests. The pod is in the namespace of the finding's resource.
type MissingRequestsPod struct {
	Name string `json:"name"`
	// Node is the node the pod is scheduled on.
	Node string `json:"node"`
	// Containers are listed in pod spec order and only when at least one
	// request is missing.
	Containers []MissingRequestsContainer `json:"containers"`
}

// MissingRequestsEvidence is the evidence for a workload whose pods have
// containers without CPU or memory requests. There is one per owning workload;
// a pod with no controller is its own workload.
type MissingRequestsEvidence struct {
	// OwnerKind and OwnerName identify the controller (Deployment,
	// StatefulSet, Job, ...); empty for a pod with no controller.
	OwnerKind string `json:"owner_kind,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`
	// PodsOnTargets is how many pods of the workload run on the target nodes
	// and would be evicted, with or without requests. It is the denominator of
	// the count in the finding's description.
	PodsOnTargets int `json:"pods_on_targets"`
	// Pods are the pods that lack requests, sorted by name then node.
	Pods []MissingRequestsPod `json:"pods"`
}

func (MissingRequestsEvidence) isEvidence() {}
