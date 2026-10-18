package analyzer

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// staleFinding is the finding for the unreconciled budget ns/name: one per
// budget, pointing at the budget, with what PDB_DISRUPTION_BLOCKED would have
// shown (numbers as last reported) and the two generations.
func staleFinding(ns, name string, healthy, desired, allowed int32, observed, generation int64, nodes []string, pods ...findings.PDBPod) findings.Finding {
	return findings.Finding{
		RuleID:   "PDB_STALE_STATUS",
		Severity: findings.Unknown,
		Resource: findings.ResourceRef{Kind: "PodDisruptionBudget", Namespace: ns, Name: name},
		Evidence: findings.PDBStaleStatusEvidence{
			PDBEvidence: findings.PDBEvidence{
				Namespace:          ns,
				Name:               name,
				CurrentHealthy:     healthy,
				DesiredHealthy:     desired,
				DisruptionsAllowed: allowed,
				Nodes:              nodes,
				Pods:               pods,
			},
			ObservedGeneration: observed,
			Generation:         generation,
		},
	}
}

func TestPDBStaleStatus(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-stale-status.yaml")
	apiPDB := func(nodes []string, pods ...findings.PDBPod) findings.Finding {
		return staleFinding("billing", "api-pdb", 2, 2, 0, 1, 2, nodes, pods...)
	}
	webPDB := staleFinding("shop", "web-pdb", 2, 1, 1, 2, 3, []string{"worker-01"}, pdbPod("shop", "web-0", "worker-01"))

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "stale budgets are reported whatever they allow; settled, DaemonSet-only and off-target budgets are not",
			targets: []string{"worker-01", "worker-02"},
			want: []findings.Finding{
				apiPDB([]string{"worker-01", "worker-02"}, pdbPod("billing", "api-0", "worker-01"), pdbPod("billing", "api-1", "worker-02")),
				webPDB,
			},
		},
		{
			name:    "only pods on the target nodes count",
			targets: []string{"worker-02"},
			want: []findings.Finding{
				apiPDB([]string{"worker-02"}, pdbPod("billing", "api-1", "worker-02")),
			},
		},
		{
			name:    "a stale budget is reported when its pod is on a target node",
			targets: []string{"worker-03"},
			want: []findings.Finding{
				staleFinding("edge", "proxy-pdb", 1, 1, 0, 1, 2, []string{"worker-03"}, pdbPod("edge", "proxy-0", "worker-03")),
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
			got := Run(nil, Input{Snapshot: s, Targets: tt.targets}, PDBStaleStatus{})

			// Ignore prose; it is covered by TestPDBStaleStatusDescribesFinding.
			for i := range got {
				got[i].Description, got[i].Remediation = "", ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

// A stale budget must show up once, as UNKNOWN: PDB_DISRUPTION_BLOCKED steps
// aside for it, and a settled budget that allows nothing stays BLOCKED. The
// two rules share the pod selection, so no budget falls between them.
func TestPDBStaleStatusReplacesBlocked(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-stale-status.yaml")

	got := Run(nil, Input{Snapshot: s, Targets: []string{"worker-01", "worker-02"}}, PDBDisruptionBlocked{}, PDBStaleStatus{})

	var summary []string
	for _, f := range got {
		summary = append(summary, fmt.Sprintf("%s %s %s", f.Severity, f.RuleID, f.Resource))
	}
	want := []string{
		"BLOCKED PDB_DISRUPTION_BLOCKED PodDisruptionBudget/batch/job-pdb",
		"UNKNOWN PDB_STALE_STATUS PodDisruptionBudget/billing/api-pdb",
		"UNKNOWN PDB_STALE_STATUS PodDisruptionBudget/shop/web-pdb",
	}
	if !slices.Equal(summary, want) {
		t.Errorf("findings\n got: %q\nwant: %q", summary, want)
	}
}

func TestPDBStaleStatusDescribesFinding(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-stale-status.yaml")

	tests := []struct {
		name    string
		targets []string
		want    string
	}{
		{
			name:    "states both generations and the pod and node counts",
			targets: []string{"worker-01", "worker-02"},
			want:    "PodDisruptionBudget billing/api-pdb has not been reconciled (status.observedGeneration 1, metadata.generation 2), so the numbers in its status are out of date and draincheck cannot tell whether eviction of its 2 pods on 2 target nodes will be refused.",
		},
		{
			name:    "singular for one pod on one node",
			targets: []string{"worker-02"},
			want:    "PodDisruptionBudget billing/api-pdb has not been reconciled (status.observedGeneration 1, metadata.generation 2), so the numbers in its status are out of date and draincheck cannot tell whether eviction of its 1 pod on 1 target node will be refused.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *findings.Finding
			for _, f := range (PDBStaleStatus{}).Evaluate(Input{Snapshot: s, Targets: tt.targets}) {
				if f.Resource.Name == "api-pdb" {
					got = &f
				}
			}

			if got == nil {
				t.Fatal("no finding for billing/api-pdb")
			}
			if got.Description != tt.want {
				t.Errorf("description\n got: %s\nwant: %s", got.Description, tt.want)
			}
			if got.Remediation == "" {
				t.Error("remediation must be set")
			}
		})
	}
}

func TestPDBStale(t *testing.T) {
	tests := []struct {
		name                 string
		generation, observed int64
		want                 bool
	}{
		{"both unset, as in a cluster that does not set them", 0, 0, false},
		{"reconciled", 2, 2, false},
		{"status behind the spec", 2, 1, true},
		{"never reconciled", 1, 0, true},
		{"status ahead of the spec still differs", 2, 3, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdb := policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{Generation: tt.generation},
				Status:     policyv1.PodDisruptionBudgetStatus{ObservedGeneration: tt.observed},
			}
			if got := pdbStale(pdb); got != tt.want {
				t.Errorf("pdbStale(generation %d, observed %d) = %v, want %v", tt.generation, tt.observed, got, tt.want)
			}
		})
	}
}

func TestPDBStaleStatusNilSnapshot(t *testing.T) {
	if got := (PDBStaleStatus{}).Evaluate(Input{Targets: []string{"worker-01"}}); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestPDBStaleStatusID(t *testing.T) {
	if got := (PDBStaleStatus{}).ID(); got != "PDB_STALE_STATUS" {
		t.Errorf("ID() = %q", got)
	}
}
