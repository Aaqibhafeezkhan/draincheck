package findings

import (
	"encoding/json"
	"testing"
)

func TestFindingJSON(t *testing.T) {
	f := Finding{
		RuleID:   "PDB_DISRUPTION_BLOCKED",
		Severity: Blocked,
		Resource: ResourceRef{Kind: "Deployment", Namespace: "billing", Name: "payments-api"},
		Evidence: PDBEvidence{Namespace: "billing", Name: "payments-api-pdb", CurrentHealthy: 3, DesiredHealthy: 3},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"rule_id":"PDB_DISRUPTION_BLOCKED","severity":"BLOCKED","resource":"Deployment/billing/payments-api","evidence":{"namespace":"billing","name":"payments-api-pdb","current_healthy":3,"desired_healthy":3,"disruptions_allowed":0},"description":"","remediation":""}`
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
