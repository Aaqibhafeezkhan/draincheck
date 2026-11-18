package findings

// The sources of a LabelMismatch.
const (
	// LabelSourceNodeSelector is a pod's spec.nodeSelector entry.
	LabelSourceNodeSelector = "node_selector"
	// LabelSourceNodeAffinity is a matchExpressions entry of a required node
	// affinity term.
	LabelSourceNodeAffinity = "node_affinity"
	// LabelSourceNodeAffinityField is a matchFields entry of a required node
	// affinity term (the scheduler supports metadata.name only).
	LabelSourceNodeAffinityField = "node_affinity_field"
)

// LabelMismatch is one requirement of a pod that a node does not meet.
type LabelMismatch struct {
	// Source is where the pod states the requirement, one of the LabelSource
	// constants.
	Source string `json:"source"`
	// Key is the label key, or the field name for node_affinity_field.
	Key string `json:"key"`
	// Operator is In, NotIn, Exists, DoesNotExist, Gt or Lt. A nodeSelector
	// entry is In with one value.
	Operator string `json:"operator"`
	// Values are the wanted values, empty for Exists and DoesNotExist.
	Values []string `json:"values"`
	// NodeHasKey says whether the node has the label (or, for a field, the
	// field); NodeValue is its value, empty when it has none.
	NodeHasKey bool   `json:"node_has_key"`
	NodeValue  string `json:"node_value"`
}

// MismatchedNode is a node that stays up and is Ready, but that none of the
// pods being moved can land on because of its labels.
type MismatchedNode struct {
	Name string `json:"name"`
	// Mismatches are the requirements that at least one moving pod does not
	// meet on this node, sorted by source, key, operator and values.
	Mismatches []LabelMismatch `json:"mismatches"`
}
