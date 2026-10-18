package analyzer

import (
	"slices"
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

func TestSelectedPDBsNothingToSelect(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-stale-status.yaml")

	if got := selectedPDBs(Input{Snapshot: s}); len(got) != 0 {
		t.Errorf("no targets: got %d selections, want 0", len(got))
	}
	if got := selectedPDBs(Input{Targets: []string{"worker-01"}}); got != nil {
		t.Errorf("nil snapshot: got %+v, want nil", got)
	}
}
