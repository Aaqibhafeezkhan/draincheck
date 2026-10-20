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

func TestFindingJSONPDBStaleStatusEvidence(t *testing.T) {
	f := Finding{
		RuleID:   "PDB_STALE_STATUS",
		Severity: Unknown,
		Resource: ResourceRef{Kind: "PodDisruptionBudget", Namespace: "billing", Name: "payments-api-pdb"},
		Evidence: PDBStaleStatusEvidence{
			PDBEvidence: PDBEvidence{
				Namespace:      "billing",
				Name:           "payments-api-pdb",
				CurrentHealthy: 3,
				DesiredHealthy: 3,
				Nodes:          []string{"worker-01"},
				Pods:           []PDBPod{{Namespace: "billing", Name: "payments-api-0", Node: "worker-01"}},
			},
			ObservedGeneration: 1,
			Generation:         2,
		},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	// The budget fields sit next to the generations, not under a nested key.
	want := `{"rule_id":"PDB_STALE_STATUS","severity":"UNKNOWN","resource":"PodDisruptionBudget/billing/payments-api-pdb","evidence":{"namespace":"billing","name":"payments-api-pdb","current_healthy":3,"desired_healthy":3,"disruptions_allowed":0,"nodes":["worker-01"],"pods":[{"namespace":"billing","name":"payments-api-0","node":"worker-01"}],"observed_generation":1,"generation":2},"description":"","remediation":""}`
	if string(b) != want {
		t.Errorf("json mismatch\n got: %s\nwant: %s", b, want)
	}
}

func TestFindingJSONOperatorManagedPDBsEvidence(t *testing.T) {
	f := Finding{
		RuleID:   "PDB_DISRUPTION_BLOCKED",
		Severity: Unknown,
		Resource: ResourceRef{Kind: "Namespace", Name: "storage-system"},
		Evidence: OperatorManagedPDBsEvidence{
			ManagedBy: ManagedBy{APIGroup: "storage.example.com", Kind: "NodeAgent"},
			Namespace: "storage-system",
			PDBs: []PDBEvidence{
				{
					Namespace:      "storage-system",
					Name:           "agent-1-pdb",
					CurrentHealthy: 1,
					DesiredHealthy: 1,
					Nodes:          []string{"worker-01"},
					Pods:           []PDBPod{{Namespace: "storage-system", Name: "agent-1", Node: "worker-01"}},
				},
			},
		},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	// One finding covers every budget of one operator in one namespace; each
	// budget keeps its own numbers, nodes and pods.
	want := `{"rule_id":"PDB_DISRUPTION_BLOCKED","severity":"UNKNOWN","resource":"Namespace/storage-system","evidence":{"managed_by":{"api_group":"storage.example.com","kind":"NodeAgent"},"namespace":"storage-system","pdbs":[{"namespace":"storage-system","name":"agent-1-pdb","current_healthy":1,"desired_healthy":1,"disruptions_allowed":0,"nodes":["worker-01"],"pods":[{"namespace":"storage-system","name":"agent-1","node":"worker-01"}]}]},"description":"","remediation":""}`
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

func TestFindingJSONPDBSyncFailedEvidence(t *testing.T) {
	f := Finding{
		RuleID:   "PDB_SYNC_FAILED",
		Severity: Unknown,
		Resource: ResourceRef{Kind: "PodDisruptionBudget", Namespace: "billing", Name: "payments-api-pdb"},
		Evidence: PDBSyncFailedEvidence{
			PDBEvidence: PDBEvidence{
				Namespace:      "billing",
				Name:           "payments-api-pdb",
				CurrentHealthy: 3,
				DesiredHealthy: 3,
				Nodes:          []string{"worker-01"},
				Pods:           []PDBPod{{Namespace: "billing", Name: "payments-api-0", Node: "worker-01"}},
			},
			Reason:  "SyncFailed",
			Message: "scale subresource not found",
		},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	// The budget fields sit next to the condition, not under a nested key.
	want := `{"rule_id":"PDB_SYNC_FAILED","severity":"UNKNOWN","resource":"PodDisruptionBudget/billing/payments-api-pdb","evidence":{"namespace":"billing","name":"payments-api-pdb","current_healthy":3,"desired_healthy":3,"disruptions_allowed":0,"nodes":["worker-01"],"pods":[{"namespace":"billing","name":"payments-api-0","node":"worker-01"}],"reason":"SyncFailed","message":"scale subresource not found"},"description":"","remediation":""}`
	if string(b) != want {
		t.Errorf("json mismatch\n got: %s\nwant: %s", b, want)
	}
}
