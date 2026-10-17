package findings

import (
	"encoding/json"
	"testing"
)

func TestFindingJSON(t *testing.T) {
	f := Finding{
		RuleID:   "PDB_DISRUPTION_BLOCKED",
		Severity: Blocked,
		Resource: ResourceRef{Kind: "PodDisruptionBudget", Namespace: "billing", Name: "payments-api-pdb"},
		Evidence: PDBEvidence{
			Namespace:      "billing",
			Name:           "payments-api-pdb",
			CurrentHealthy: 3,
			DesiredHealthy: 3,
			Nodes:          []string{"worker-01", "worker-02"},
			Pods: []PDBPod{
				{Namespace: "billing", Name: "payments-api-0", Node: "worker-01"},
				{Namespace: "billing", Name: "payments-api-1", Node: "worker-02"},
			},
		},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"rule_id":"PDB_DISRUPTION_BLOCKED","severity":"BLOCKED","resource":"PodDisruptionBudget/billing/payments-api-pdb","evidence":{"namespace":"billing","name":"payments-api-pdb","current_healthy":3,"desired_healthy":3,"disruptions_allowed":0,"nodes":["worker-01","worker-02"],"pods":[{"namespace":"billing","name":"payments-api-0","node":"worker-01"},{"namespace":"billing","name":"payments-api-1","node":"worker-02"}]},"description":"","remediation":""}`
	if string(b) != want {
		t.Errorf("json mismatch\n got: %s\nwant: %s", b, want)
	}
}

func TestFindingJSONClusterScopedNilEvidence(t *testing.T) {
	f := Finding{
		RuleID:   "R",
		Severity: Unknown,
		Resource: ResourceRef{Kind: "Node", Name: "worker-02"},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"rule_id":"R","severity":"UNKNOWN","resource":"Node/worker-02","evidence":null,"description":"","remediation":""}`
	if string(b) != want {
		t.Errorf("json mismatch\n got: %s\nwant: %s", b, want)
	}
}
