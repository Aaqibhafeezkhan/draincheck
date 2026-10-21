package findings

// SingleReplicaPod identifies one pod of a single-replica workload that sits on
// a target node. It has the fields every pod entry in evidence has (namespace,
// name and node, as PDBPod and OwnershipPod); issue #86 replaces them by one
// shared type.
type SingleReplicaPod struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// Node is the target node the pod runs on.
	Node string `json:"node"`
}

// SingleReplicaEvidence is the evidence for a workload that desires one
// replica and has a pod on a target node.
type SingleReplicaEvidence struct {
	// OwnerKind and OwnerName identify the Deployment or StatefulSet; its
	// namespace is the namespace of the finding's resource.
	OwnerKind string `json:"owner_kind"`
	OwnerName string `json:"owner_name"`
	// Replicas is the desired replica count, spec.replicas (1 when unset).
	Replicas int32 `json:"replicas"`
	// Pods are the workload's pods on the target nodes, sorted by name. More
	// than one can exist while a rollout is in progress.
	Pods []SingleReplicaPod `json:"pods"`
}

func (SingleReplicaEvidence) isEvidence() {}
