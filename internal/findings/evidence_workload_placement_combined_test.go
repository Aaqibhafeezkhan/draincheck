package findings

import (
	"encoding/json"
	"testing"
)

func TestFindingJSONWorkloadPlacementCombinedEvidence(t *testing.T) {
	f := Finding{
		RuleID:   "WORKLOAD_PLACEMENT_COMBINED",
		Severity: Risk,
		Resource: ResourceRef{Kind: "StatefulSet", Namespace: "shop", Name: "split"},
		Evidence: WorkloadPlacementCombinedEvidence{
			OwnerKind: "StatefulSet", OwnerName: "split",
			Pods:           []StrandedPod{{Name: "split-0", Node: "worker"}},
			RemainingNodes: 2,
			RuledOutNodes: []RuledOutNode{
				{Name: "gpu", Taints: []NodeTaint{{Key: "dedicated", Effect: "NoSchedule"}}, Mismatches: []LabelMismatch{}},
				{Name: "plain", Taints: []NodeTaint{}, Mismatches: []LabelMismatch{
					{Source: "node_selector", Key: "disktype", Operator: "In", Values: []string{"ssd"}, NodeHasKey: true, NodeValue: "hdd"}}},
			},
		},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"rule_id":"WORKLOAD_PLACEMENT_COMBINED","severity":"RISK","resource":"StatefulSet/shop/split","evidence":{"owner_kind":"StatefulSet","owner_name":"split","pods":[{"name":"split-0","node":"worker"}],"remaining_nodes":2,"ruled_out_nodes":[{"name":"gpu","taints":[{"key":"dedicated","effect":"NoSchedule"}],"mismatches":[]},{"name":"plain","taints":[],"mismatches":[{"source":"node_selector","key":"disktype","operator":"In","values":["ssd"],"node_has_key":true,"node_value":"hdd"}]}]},"case":"","description":""}`
	if string(b) != want {
		t.Errorf("json mismatch\n got: %s\nwant: %s", b, want)
	}
}
