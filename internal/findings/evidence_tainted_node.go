package findings

// NodeTaint is a taint that keeps pods off a node.
type NodeTaint struct {
	Key    string `json:"key"`
	Effect string `json:"effect"`
}

// TaintedNode is a node that stays up and is Ready, but that none of the pods
// being moved can land on because of its taints.
type TaintedNode struct {
	Name string `json:"name"`
	// Taints are the NoSchedule and NoExecute taints that at least one moving
	// pod does not tolerate, in the order the node lists them.
	Taints []NodeTaint `json:"taints"`
}
