package findings

import (
	"encoding/json"
	"testing"
)

func TestFindingJSONWorkloadUnmatchedLabelsEvidence(t *testing.T) {
	f := Finding{
		RuleID:   "WORKLOAD_UNMATCHED_NODE_LABELS",
		Severity: Risk,
		Resource: ResourceRef{Kind: "StatefulSet", Namespace: "shop", Name: "cache"},
		Evidence: WorkloadUnmatchedLabelsEvidence{
			OwnerKind: "StatefulSet", OwnerName: "cache",
			Pods:           []StrandedPod{{Name: "cache-0", Node: "worker"}},
			RemainingNodes: 1,
			MismatchedNodes: []MismatchedNode{{Name: "node-a", Mismatches: []LabelMismatch{
				{Source: "node_selector", Key: "disktype", Operator: "In", Values: []string{"ssd"}, NodeHasKey: true, NodeValue: "hdd"}}}},
		},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"rule_id":"WORKLOAD_UNMATCHED_NODE_LABELS","severity":"RISK","resource":"StatefulSet/shop/cache","evidence":{"owner_kind":"StatefulSet","owner_name":"cache","pods":[{"name":"cache-0","node":"worker"}],"remaining_nodes":1,"mismatched_nodes":[{"name":"node-a","mismatches":[{"source":"node_selector","key":"disktype","operator":"In","values":["ssd"],"node_has_key":true,"node_value":"hdd"}]}]},"case":"","description":""}`
	if string(b) != want {
		t.Errorf("json mismatch\n got: %s\nwant: %s", b, want)
	}
}
