package findings

import (
	"encoding/json"
	"testing"
)

func TestSeverityValid(t *testing.T) {
	for _, s := range []Severity{Blocked, Risk, Unknown} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range []Severity{"SAFE", ""} {
		if s.Valid() {
			t.Errorf("%q should be invalid", s)
		}
	}
}

func TestResourceRefString(t *testing.T) {
	tests := []struct {
		ref  ResourceRef
		want string
	}{
		{ResourceRef{Kind: "Deployment", Namespace: "billing", Name: "payments-api"}, "Deployment/billing/payments-api"},
		{ResourceRef{Kind: "Node", Name: "worker-02"}, "Node/worker-02"},
	}
	for _, tt := range tests {
		if got := tt.ref.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

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
