package findings

// UnschedulableNode is a node that stays up but takes no new pods.
type UnschedulableNode struct {
	Name string `json:"name"`
	// Cordoned is spec.unschedulable.
	Cordoned bool `json:"cordoned"`
	// ReadyStatus is the status of the node's Ready condition: "True", "False"
	// or "Unknown". Empty when the node reports no Ready condition.
	ReadyStatus string `json:"ready_status,omitempty"`
}

// NodeNotSchedulableEvidence is the evidence for the nodes outside the target
// set that evicted pods cannot land on. There is one finding for all of them,
// because the cause is one: the room left after the drain is smaller than the
// node count suggests.
type NodeNotSchedulableEvidence struct {
	// PodsToMove is the number of pods on the target nodes that a drain evicts
	// and that a controller recreates elsewhere.
	PodsToMove int `json:"pods_to_move"`
	// RemainingNodes is the number of nodes outside the target set, whatever
	// their state.
	RemainingNodes int `json:"remaining_nodes"`
	// Nodes are the remaining nodes that take no new pods, sorted by name.
	Nodes []UnschedulableNode `json:"nodes"`
}

func (NodeNotSchedulableEvidence) isEvidence() {}
