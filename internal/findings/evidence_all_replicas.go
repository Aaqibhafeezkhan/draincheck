package findings

// AllReplicasPod identifies one Ready pod of a workload that sits on a target
// node. It has the fields every pod entry in evidence has (namespace, name and
// node); issue #86 replaces them by one shared type.
type AllReplicasPod struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// Node is the target node the pod runs on.
	Node string `json:"node"`
}

// AllReplicasEvidence is the evidence for a workload whose Ready pods all run
// on target nodes.
type AllReplicasEvidence struct {
	// OwnerKind and OwnerName identify the Deployment or StatefulSet; its
	// namespace is the namespace of the finding's resource.
	OwnerKind string `json:"owner_kind"`
	OwnerName string `json:"owner_name"`
	// Replicas is the desired replica count, spec.replicas (1 when unset).
	Replicas int32 `json:"replicas"`
	// ReadyPods is the number of Ready pods the workload has, all on target
	// nodes. It can be lower than Replicas while a rollout or a restart is
	// in progress.
	ReadyPods int `json:"ready_pods"`
	// Nodes are the target nodes the Ready pods run on, sorted, without
	// duplicates.
	Nodes []string `json:"nodes"`
	// Pods are the Ready pods, sorted by namespace, name and node.
	Pods []AllReplicasPod `json:"pods"`
}

func (AllReplicasEvidence) isEvidence() {}
