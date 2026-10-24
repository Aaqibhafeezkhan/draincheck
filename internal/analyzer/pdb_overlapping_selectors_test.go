package analyzer

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func TestPDBOverlappingSelectors(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-overlapping-selectors.yaml")
	// Every budget of the fixture allows 1 disruption and has no other number.
	allowing := func(names ...string) []findings.OverlappingPDB {
		var out []findings.OverlappingPDB
		for _, n := range names {
			out = append(out, findings.OverlappingPDB{Name: n, DisruptionsAllowed: 1})
		}
		return out
	}
	web := findings.OverlappingPDBsEvidence{Namespace: "shop", PDBs: []string{"web-app", "web-tier"}, Budgets: allowing("web-app", "web-tier")}
	db := findings.OverlappingPDBsEvidence{Namespace: "shop", PDBs: []string{"db-a", "db-b", "db-c"}, Budgets: allowing("db-a", "db-b", "db-c")}
	etl := findings.OverlappingPDBsEvidence{Namespace: "batch", PDBs: []string{"etl", "everything"}, Budgets: allowing("etl", "everything")}

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "pod matched by two budgets, and by three listed in name order; DaemonSet and static pods are not reported",
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
			got := Run(nil, Input{Snapshot: s, Targets: tt.targets}, PDBOverlappingSelectors{})

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
	// Checked on a kind cluster: kubectl drain gets the refusal as an error for
	// the pod and does not retry it, unlike a budget at 0.
	if !strings.Contains(got[0].Description, "kubectl drain reports this error for the pod at once and does not retry it") {
		t.Errorf("description should say what kubectl drain does: %s", got[0].Description)
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

// The numbers the overlapping budgets report are kept in the evidence, so the
// operator sees them next to the statement that they cannot be trusted (#139).
func TestPDBOverlappingSelectorsKeepsBudgetNumbers(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-overlapping-numbers.yaml")

	got := PDBOverlappingSelectors{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}})

	var web *findings.Finding
	for i := range got {
		if got[i].Resource.Name == "web-0" {
			web = &got[i]
		}
	}
	if web == nil {
		t.Fatalf("no finding for web-0 in %+v", got)
	}
	want := findings.OverlappingPDBsEvidence{
		Namespace: "shop",
		PDBs:      []string{"dup-a", "dup-b"},
		Budgets: []findings.OverlappingPDB{
			{Name: "dup-a", CurrentHealthy: 2, DesiredHealthy: 1, DisruptionsAllowed: 1},
			{Name: "dup-b", CurrentHealthy: 0, DesiredHealthy: 1, DisruptionsAllowed: 0},
		},
	}
	if !reflect.DeepEqual(web.Evidence, want) {
		t.Errorf("evidence mismatch\n got: %+v\nwant: %+v", web.Evidence, want)
	}
	for _, phrase := range []string{
		"The disruption controller chooses one of them arbitrarily when a pod changes",
		"their numbers can be out of date",
		"dup-a 2 healthy, 1 desired, 1 allowed",
		"dup-b 0 healthy, 1 desired, 0 allowed",
	} {
		if !strings.Contains(web.Description, phrase) {
			t.Errorf("description should contain %q: %s", phrase, web.Description)
		}
	}
}
