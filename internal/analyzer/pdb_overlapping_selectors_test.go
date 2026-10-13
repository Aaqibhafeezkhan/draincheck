package analyzer

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func TestPDBOverlappingSelectors(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-overlapping-selectors.yaml")
	web := findings.OverlappingPDBsEvidence{Namespace: "shop", PDBs: []string{"web-app", "web-tier"}}
	db := findings.OverlappingPDBsEvidence{Namespace: "shop", PDBs: []string{"db-a", "db-b", "db-c"}}
	etl := findings.OverlappingPDBsEvidence{Namespace: "batch", PDBs: []string{"etl", "everything"}}

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "pod matched by two budgets, and by three listed in name order",
			targets: []string{"worker-01"},
			want: []findings.Finding{
				{RuleID: "PDB_OVERLAPPING_SELECTORS", Severity: findings.Blocked, Resource: findings.ResourceRef{Kind: "Pod", Namespace: "shop", Name: "db-0"}, Evidence: db},
				{RuleID: "PDB_OVERLAPPING_SELECTORS", Severity: findings.Blocked, Resource: findings.ResourceRef{Kind: "Pod", Namespace: "shop", Name: "web-0"}, Evidence: web},
			},
		},
		{
			name:    "pod with one budget, and an empty selector overlapping a specific one",
			targets: []string{"worker-02"},
			want: []findings.Finding{
				{RuleID: "PDB_OVERLAPPING_SELECTORS", Severity: findings.Blocked, Resource: findings.ResourceRef{Kind: "Pod", Namespace: "batch", Name: "etl-0"}, Evidence: etl},
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
			got := Run(Input{Snapshot: s, Targets: tt.targets}, PDBOverlappingSelectors{})

			// Ignore prose; it is covered by TestPDBOverlappingSelectorsDescribesFinding.
			for i := range got {
				got[i].Description, got[i].Remediation = "", ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

func TestPDBOverlappingSelectorsDescribesFinding(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-overlapping-selectors.yaml")

	got := PDBOverlappingSelectors{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-02"}})

	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1", len(got))
	}
	if got[0].Description == "" || got[0].Remediation == "" {
		t.Errorf("description and remediation must be set: %+v", got[0])
	}
	for _, name := range []string{"etl", "everything", "etl-0"} {
		if !strings.Contains(got[0].Description, name) {
			t.Errorf("description should mention %q: %s", name, got[0].Description)
		}
	}
}

func TestPDBOverlappingSelectorsNilSnapshot(t *testing.T) {
	if got := (PDBOverlappingSelectors{}).Evaluate(Input{Targets: []string{"worker-01"}}); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestPDBOverlappingSelectorsID(t *testing.T) {
	if got := (PDBOverlappingSelectors{}).ID(); got != "PDB_OVERLAPPING_SELECTORS" {
		t.Errorf("ID() = %q", got)
	}
}
