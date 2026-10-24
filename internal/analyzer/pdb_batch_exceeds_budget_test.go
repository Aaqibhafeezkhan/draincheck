package analyzer

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

// batchFinding is the finding for the budget ns/name that allows `allowed`
// disruptions while the batch evicts the given pods: one per budget, pointing
// at the budget.
func batchFinding(ns, name string, healthy, desired, allowed int32, nodes []string, pods ...findings.PDBPod) findings.Finding {
	return findings.Finding{
		RuleID:   "PDB_BATCH_EXCEEDS_BUDGET",
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: "PodDisruptionBudget", Namespace: ns, Name: name},
		Evidence: findings.PDBBatchEvidence{
			PDBEvidence: findings.PDBEvidence{
				Namespace:          ns,
				Name:               name,
				CurrentHealthy:     healthy,
				DesiredHealthy:     desired,
				DisruptionsAllowed: allowed,
				Nodes:              nodes,
				Pods:               pods,
			},
			PodsToEvict: len(pods),
		},
	}
}

func TestPDBBatchExceedsBudget(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-batch-exceeds-budget.yaml")
	web := batchFinding("shop", "web-pdb", 2, 1, 1, []string{"worker-01", "worker-02"},
		pdbPod("shop", "web-0", "worker-01"), pdbPod("shop", "web-1", "worker-02"))
	api := batchFinding("billing", "api-pdb", 3, 1, 2, []string{"worker-01", "worker-02", "worker-03"},
		pdbPod("billing", "api-0", "worker-01"), pdbPod("billing", "api-1", "worker-02"), pdbPod("billing", "api-2", "worker-03"))

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "each node alone fits the budget, the batch does not; budgets that fit, are at 0, stale, failed, operator-managed or DaemonSet-only are not reported",
			targets: []string{"worker-01", "worker-02"},
			want:    []findings.Finding{web},
		},
		{
			name:    "a budget that allows 2 is exceeded by 3 pods",
			targets: []string{"worker-01", "worker-02", "worker-03"},
			want:    []findings.Finding{api, web},
		},
		{
			name:    "one node alone fits every budget",
			targets: []string{"worker-01"},
			want:    nil,
		},
		{
			name:    "no targets",
			targets: nil,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Run(nil, Input{Snapshot: s, Targets: tt.targets}, PDBBatchExceedsBudget{})

			// Ignore prose; it is covered by TestPDBBatchExceedsBudgetDescribesFinding.
			for i := range got {
				got[i].Description, got[i].Remediation = "", ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

// A budget at 0 is the stall case and stays with PDB_DISRUPTION_BLOCKED; a
// budget that allows some disruptions but not the whole batch is this rule's.
// The two never report the same budget.
func TestPDBBatchExceedsBudgetLeavesZeroToBlocked(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-batch-exceeds-budget.yaml")

	got := Run(nil, Input{Snapshot: s, Targets: []string{"worker-01", "worker-02"}}, PDBDisruptionBlocked{}, PDBBatchExceedsBudget{})

	var summary []string
	for _, f := range got {
		summary = append(summary, fmt.Sprintf("%s %s %s", f.Severity, f.RuleID, f.Resource))
	}
	want := []string{
		"BLOCKED PDB_DISRUPTION_BLOCKED PodDisruptionBudget/batch/job-pdb",
		"RISK PDB_BATCH_EXCEEDS_BUDGET PodDisruptionBudget/shop/web-pdb",
	}
	if !slices.Equal(summary, want) {
		t.Errorf("findings\n got: %q\nwant: %q", summary, want)
	}
}

func TestPDBBatchExceedsBudgetDescribesFinding(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-batch-exceeds-budget.yaml")

	tests := []struct {
		name    string
		targets []string
		pdb     string
		want    string
	}{
		{
			name:    "states pods, nodes and how many the budget allows at once",
			targets: []string{"worker-01", "worker-02", "worker-03"},
			pdb:     "api-pdb",
			want:    "Draining 3 target nodes evicts 3 pods selected by PodDisruptionBudget billing/api-pdb, which allows 2 disruptions at once, so the drain will wait for replacements to become Ready between evictions.",
		},
		{
			name:    "singular for one disruption",
			targets: []string{"worker-01", "worker-02"},
			pdb:     "web-pdb",
			want:    "Draining 2 target nodes evicts 2 pods selected by PodDisruptionBudget shop/web-pdb, which allows 1 disruption at once, so the drain will wait for replacements to become Ready between evictions.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *findings.Finding
			for _, f := range (PDBBatchExceedsBudget{}).Evaluate(Input{Snapshot: s, Targets: tt.targets}) {
				if f.Resource.Name == tt.pdb {
					got = &f
				}
			}
			if got == nil {
				t.Fatalf("no finding for %s", tt.pdb)
			}
			if got.Description != tt.want {
				t.Errorf("description\n got: %s\nwant: %s", got.Description, tt.want)
			}
			if got.Remediation == "" {
				t.Error("remediation is empty")
			}
		})
	}
}

// A pod the eviction API lets past its budget does not use the budget up, so
// it is not counted: cnt-pdb allows 2 and three pods are on the targets, but
// one is not Ready and AlwaysAllow, so the batch fits. cnt2-pdb allows 1 and
// is exceeded by the two pods that do use the budget, not by three.
func TestPDBBatchExceedsBudgetIgnoresUnhealthyEvictions(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-unhealthy-pods.yaml")

	got := Run(nil, Input{Snapshot: s, Targets: []string{"worker-01", "worker-02", "worker-03"}}, PDBBatchExceedsBudget{})
	for i := range got {
		got[i].Description, got[i].Remediation = "", ""
	}

	want := batchFinding("cnt2", "cnt2-pdb", 3, 2, 1, []string{"worker-01", "worker-02"},
		pdbPod("cnt2", "cnt2-0", "worker-01"), pdbPod("cnt2", "cnt2-1", "worker-02"))
	ev := want.Evidence.(findings.PDBBatchEvidence)
	ev.UnhealthyPodEvictionPolicy = "AlwaysAllow"
	ev.EvictableUnhealthyPods = []findings.PDBPod{pdbPod("cnt2", "cnt2-2", "worker-03")}
	want.Evidence = ev

	if !reflect.DeepEqual(got, []findings.Finding{want}) {
		t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, []findings.Finding{want})
	}
}

// dup-a allows 1 and two pods are evicted, which would be a finding, but dup-a
// overlaps dup-b and its numbers are whichever sync last picked it (#139), so
// PDB_OVERLAPPING_SELECTORS reports the pods and this rule does not report the
// budget. api-pdb is not overlapped and is reported.
func TestPDBBatchExceedsBudgetSkipsOverlappingPDBs(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-overlapping-numbers.yaml")

	got := Run(nil, Input{Snapshot: s, Targets: []string{"worker-01", "worker-02"}}, PDBBatchExceedsBudget{})

	for i := range got {
		got[i].Description, got[i].Remediation = "", ""
	}
	want := []findings.Finding{
		batchFinding("shop", "api-pdb", 2, 1, 1, []string{"worker-01", "worker-02"},
			pdbPod("shop", "api-0", "worker-01"), pdbPod("shop", "api-1", "worker-02")),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, want)
	}
}
