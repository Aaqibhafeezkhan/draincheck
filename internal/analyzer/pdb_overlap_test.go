package analyzer

import (
	"slices"
	"testing"

	"k8s.io/apimachinery/pkg/types"
)

// overlappedNames lists the budgets overlappedPDBs returns as namespace/name,
// sorted.
func overlappedNames(in Input) []string {
	var out []string
	for key := range overlappedPDBs(in) {
		out = append(out, key.String())
	}
	slices.Sort(out)
	return out
}

func TestOverlappedPDBs(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-overlapping-numbers.yaml")

	tests := []struct {
		name    string
		targets []string
		want    []string
	}{
		{
			name:    "budgets that select the same evicted pod are overlapped, others are not",
			targets: []string{"worker-01", "worker-02"},
			want:    []string{"shop/dup-a", "shop/dup-b", "shop/side-a", "shop/side-b"},
		},
		{
			name:    "an overlap that only reaches pods off the targets does not count",
			targets: []string{"worker-02"},
			want:    []string{"shop/dup-a", "shop/dup-b"},
		},
		{
			name:    "no targets",
			targets: nil,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := overlappedNames(Input{Snapshot: s, Targets: tt.targets})
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// The overlap rule and the budgets it makes untrustworthy must come from one
// definition: every budget named by a PDB_OVERLAPPING_SELECTORS finding is
// overlapped, and no other is.
func TestOverlappedPDBsMatchOverlappingSelectors(t *testing.T) {
	for _, fixture := range []string{"pdb-overlapping-selectors.yaml", "pdb-overlapping-numbers.yaml"} {
		s := snapshotFromFixture(t, fixture)
		for _, targets := range [][]string{{"worker-01"}, {"worker-02"}, {"worker-01", "worker-02"}} {
			in := Input{Snapshot: s, Targets: targets}
			var want []string
			for _, o := range overlaps(in) {
				for _, pdb := range o.pdbs {
					want = append(want, types.NamespacedName{Namespace: pdb.Namespace, Name: pdb.Name}.String())
				}
			}
			slices.Sort(want)
			want = slices.Compact(want)
			if got := overlappedNames(in); !slices.Equal(got, want) {
				t.Errorf("%s %v: overlapped %v, overlapping pods name %v", fixture, targets, got, want)
			}
		}
	}
}

func TestOverlappedPDBsNilSnapshot(t *testing.T) {
	if got := overlappedPDBs(Input{Targets: []string{"worker-01"}}); len(got) != 0 {
		t.Errorf("got %v, want none", got)
	}
}
