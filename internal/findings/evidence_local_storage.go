package findings

// LocalStoragePod identifies one pod with emptyDir volumes and names them.
type LocalStoragePod struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// Node is the target node the pod runs on.
	Node string `json:"node"`
	// Volumes are the names of the pod's emptyDir volumes, sorted.
	Volumes []string `json:"volumes"`
}

// LocalStorageEvidence is the evidence for pods with emptyDir volumes on a
// target node. One finding covers the pods of one workload, or one pod with no
// controller.
type LocalStorageEvidence struct {
	// OwnerKind and OwnerName identify the controller; empty for a pod with no
	// controller.
	OwnerKind string `json:"owner_kind,omitempty"`
	OwnerName string `json:"owner_name,omitempty"`
	// DeleteEmptyDirData is whether --delete-emptydir-data was given: false for
	// the BLOCKED finding, true for the RISK one.
	DeleteEmptyDirData bool `json:"delete_emptydir_data"`
	// Nodes are the nodes the covered pods run on, sorted, without duplicates.
	Nodes []string `json:"nodes"`
	// Pods are all pods the finding covers, sorted by namespace, name and node.
	Pods []LocalStoragePod `json:"pods"`
}

func (LocalStorageEvidence) isEvidence() {}
