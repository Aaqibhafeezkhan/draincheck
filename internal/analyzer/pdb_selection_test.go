package analyzer

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func TestSelectedPDBs(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-stale-status.yaml")

	got := selectedPDBs(Input{Snapshot: s, Targets: []string{"worker-01", "worker-02"}})

	// Every budget that selects a pod kubectl would evict on a target node,
	// whatever its status says. agent-pdb (DaemonSet pod only) and proxy-pdb
	// (pod off the target nodes) select nothing that counts.
	var names []string
	byName := make(map[string]pdbSelection)
	for _, sel := range got {
		name := sel.pdb.Namespace + "/" + sel.pdb.Name
		names = append(names, name)
		byName[name] = sel
	}
	slices.Sort(names)
	if want := []string{"batch/job-pdb", "billing/api-pdb", "shop/cache-pdb", "shop/web-pdb"}; !slices.Equal(names, want) {
		t.Fatalf("selected budgets = %v, want %v", names, want)
	}

	api := byName["billing/api-pdb"]
	if want := []string{"worker-01", "worker-02"}; !slices.Equal(api.nodes, want) {
		t.Errorf("api-pdb nodes = %v, want %v", api.nodes, want)
	}
	if want := []findings.PDBPod{pdbPod("billing", "api-0", "worker-01"), pdbPod("billing", "api-1", "worker-02")}; !slices.Equal(api.pods, want) {
		t.Errorf("api-pdb pods = %v, want %v", api.pods, want)
	}
}

// operators are the custom-resource controllers of the pods a budget selects on
// the target nodes: plain owners (Deployment, StatefulSet, Job, ...) and pods
// off the target nodes do not count, and each operator is listed once.
func TestSelectedPDBsOperators(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-operator-managed.yaml")
	nodeAgent := findings.ManagedBy{APIGroup: "storage.example.com", Kind: "NodeAgent"}

	plain := []string{"shop/web-pdb", "shop/db-pdb", "shop/migrate-pdb", "shop/legacy-pdb"}
	tests := []struct {
		name    string
		targets []string
		want    map[string][]findings.ManagedBy // by "namespace/name"
		none    []string                        // budgets that must be selected with no operator
	}{
		{
			name:    "all nodes",
			targets: []string{"worker-01", "worker-02", "worker-03", "worker-04", "worker-05"},
			want: map[string][]findings.ManagedBy{
				"storage-system/agent-1-pdb":       {nodeAgent},
				"mixed/mixed-pdb":                  {nodeAgent}, // its Deployment pod adds nothing
				"block-storage/instance-manager-a": {{APIGroup: "longhorn.io", Kind: "InstanceManager"}},
				// Two custom owners behind one budget: both listed, sorted by group then kind.
				"twin/twin-pdb": {{APIGroup: "other.example.org", Kind: "Gadget"}, {APIGroup: "other.example.org", Kind: "Widget"}},
			},
			none: plain,
		},
		{
			name:    "the operator pod is not on a target node",
			targets: []string{"worker-01"},
			want: map[string][]findings.ManagedBy{
				"storage-system/agent-1-pdb":       {nodeAgent},
				"block-storage/instance-manager-a": {{APIGroup: "longhorn.io", Kind: "InstanceManager"}},
			},
			// mixed-pdb is selected through its Deployment pod only.
			none: []string{"mixed/mixed-pdb", "shop/web-pdb", "shop/migrate-pdb"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := make(map[string][]findings.ManagedBy)
			for _, sel := range selectedPDBs(Input{Snapshot: s, Targets: tt.targets}) {
				got[sel.pdb.Namespace+"/"+sel.pdb.Name] = sel.operators
			}
			for key, want := range tt.want {
				if g, ok := got[key]; !ok || !slices.Equal(g, want) {
					t.Errorf("%s: operators = %v (selected: %v), want %v", key, g, ok, want)
				}
			}
			for _, key := range tt.none {
				if g, ok := got[key]; !ok || len(g) != 0 {
					t.Errorf("%s: operators = %v (selected: %v), want selected with none", key, g, ok)
				}
			}
		})
	}
}

func TestSelectedPDBsNothingToSelect(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-stale-status.yaml")

	if got := selectedPDBs(Input{Snapshot: s}); len(got) != 0 {
		t.Errorf("no targets: got %d selections, want 0", len(got))
	}
	if got := selectedPDBs(Input{Targets: []string{"worker-01"}}); got != nil {
		t.Errorf("nil snapshot: got %+v, want nil", got)
	}
}

// --disable-eviction: kubectl deletes the pods, so no budget is consulted and
// every PDB rule is silent, whatever the budgets say. Each case first shows
// that the rule reports something without the flag, so the silence is the
// flag's doing and not an empty fixture.
func TestPDBRulesAreSilentWithDisabledEviction(t *testing.T) {
	tests := []struct {
		rule    Rule
		fixture string
		targets []string
	}{
		{PDBDisruptionBlocked{}, "pdb-disruption-blocked.yaml", []string{"worker-01"}},
		{PDBOverlappingSelectors{}, "pdb-overlapping-selectors.yaml", []string{"worker-01"}},
		{PDBBatchExceedsBudget{}, "pdb-batch-exceeds-budget.yaml", []string{"worker-01", "worker-02"}},
		{PDBStaleStatus{}, "pdb-stale-status.yaml", []string{"worker-01", "worker-02"}},
		{PDBSyncFailed{}, "pdb-sync-failed.yaml", []string{"worker-01", "worker-02"}},
	}
	for _, tt := range tests {
		t.Run(tt.rule.ID(), func(t *testing.T) {
			s := snapshotFromFixture(t, tt.fixture)

			if got := Run(nil, Input{Snapshot: s, Targets: tt.targets}, tt.rule); len(got) == 0 {
				t.Fatal("the rule reports nothing without the flag; the case proves nothing")
			}
			got := Run(nil, Input{Snapshot: s, Targets: tt.targets, Drain: DrainOptions{DisableEviction: true}}, tt.rule)
			if len(got) != 0 {
				t.Errorf("findings with --disable-eviction = %+v, want none", got)
			}
		})
	}
}

// The shared selection is where the flag acts, so a rule added later that
// builds on it is silent too.
func TestSelectedPDBsAndOverlapsWithDisabledEviction(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-overlapping-selectors.yaml")
	in := Input{Snapshot: s, Targets: []string{"worker-01", "worker-02"}}
	if len(selectedPDBs(in)) == 0 || len(overlaps(in)) == 0 {
		t.Fatal("the fixture selects nothing without the flag; the case proves nothing")
	}

	in.Drain.DisableEviction = true
	if got := selectedPDBs(in); len(got) != 0 {
		t.Errorf("selectedPDBs = %d budgets, want none", len(got))
	}
	if got := overlaps(in); len(got) != 0 {
		t.Errorf("overlaps = %d pods, want none", len(got))
	}
	if got := overlappedPDBs(in); len(got) != 0 {
		t.Errorf("overlappedPDBs = %v, want none", got)
	}
}

func TestSelectedPDBsWithDisabledEvictionNilSnapshot(t *testing.T) {
	in := Input{Targets: []string{"worker-01"}, Drain: DrainOptions{DisableEviction: true}}
	if got := selectedPDBs(in); got != nil {
		t.Errorf("selectedPDBs = %v, want nil", got)
	}
}

// The flag silences the budget rules and nothing else: the rest of the report
// is the same with and without it.
func TestDisabledEvictionLeavesOtherRulesAlone(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-disruption-blocked.yaml")
	targets := []string{"worker-01", "worker-02"}
	notPDB := func(fs []findings.Finding) []findings.Finding {
		var out []findings.Finding
		for _, f := range fs {
			if !strings.HasPrefix(f.RuleID, "PDB_") {
				out = append(out, f)
			}
		}
		return out
	}

	without := Run(nil, Input{Snapshot: s, Targets: targets}, DefaultRules()...)
	with := Run(nil, Input{Snapshot: s, Targets: targets, Drain: DrainOptions{DisableEviction: true}}, DefaultRules()...)

	if len(notPDB(without)) == len(without) {
		t.Fatal("no PDB finding without the flag; the case proves nothing")
	}
	if len(notPDB(with)) != len(with) {
		t.Errorf("PDB findings with --disable-eviction: %+v", with)
	}
	if !reflect.DeepEqual(notPDB(without), notPDB(with)) {
		t.Errorf("other rules changed with --disable-eviction\nwithout: %+v\nwith:    %+v", notPDB(without), notPDB(with))
	}
}
