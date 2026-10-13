package analyzer

import (
	"reflect"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func TestPDBDisruptionBlocked(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-disruption-blocked.yaml")
	apiPDB := findings.PDBEvidence{Namespace: "billing", Name: "api-pdb", CurrentHealthy: 2, DesiredHealthy: 2}
	allPDB := findings.PDBEvidence{Namespace: "batch", Name: "all-pods", CurrentHealthy: 1, DesiredHealthy: 1}

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "pods on the target node covered by a zero-disruption PDB",
			targets: []string{"worker-01"},
			want: []findings.Finding{
				{RuleID: "PDB_DISRUPTION_BLOCKED", Severity: findings.Blocked, Resource: findings.ResourceRef{Kind: "Pod", Namespace: "batch", Name: "job-0"}, Evidence: allPDB},
				{RuleID: "PDB_DISRUPTION_BLOCKED", Severity: findings.Blocked, Resource: findings.ResourceRef{Kind: "Pod", Namespace: "billing", Name: "api-0"}, Evidence: apiPDB},
			},
		},
		{
			name:    "only pods on target nodes count",
			targets: []string{"worker-02"},
			want: []findings.Finding{
				{RuleID: "PDB_DISRUPTION_BLOCKED", Severity: findings.Blocked, Resource: findings.ResourceRef{Kind: "Pod", Namespace: "billing", Name: "api-1"}, Evidence: apiPDB},
			},
		},
		{
			name:    "every target node reports its own pods",
			targets: []string{"worker-01", "worker-02"},
			want: []findings.Finding{
				{RuleID: "PDB_DISRUPTION_BLOCKED", Severity: findings.Blocked, Resource: findings.ResourceRef{Kind: "Pod", Namespace: "batch", Name: "job-0"}, Evidence: allPDB},
				{RuleID: "PDB_DISRUPTION_BLOCKED", Severity: findings.Blocked, Resource: findings.ResourceRef{Kind: "Pod", Namespace: "billing", Name: "api-0"}, Evidence: apiPDB},
				{RuleID: "PDB_DISRUPTION_BLOCKED", Severity: findings.Blocked, Resource: findings.ResourceRef{Kind: "Pod", Namespace: "billing", Name: "api-1"}, Evidence: apiPDB},
			},
		},
		{
			name:    "no targets",
			targets: nil,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Run(Input{Snapshot: s, Targets: tt.targets}, PDBDisruptionBlocked{})

			// Ignore prose; it is covered by TestPDBDisruptionBlockedDescribesFinding.
			for i := range got {
				got[i].Description, got[i].Remediation = "", ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

func TestPDBDisruptionBlockedDescribesFinding(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-disruption-blocked.yaml")

	got := PDBDisruptionBlocked{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-02"}})

	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1", len(got))
	}
	if got[0].Description == "" || got[0].Remediation == "" {
		t.Errorf("description and remediation must be set: %+v", got[0])
	}
}

func TestPDBDisruptionBlockedID(t *testing.T) {
	if got := (PDBDisruptionBlocked{}).ID(); got != "PDB_DISRUPTION_BLOCKED" {
		t.Errorf("ID() = %q", got)
	}
}
