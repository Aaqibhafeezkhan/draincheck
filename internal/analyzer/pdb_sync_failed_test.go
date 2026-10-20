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

// syncFailedFinding is the finding for the budget ns/name whose controller
// failed: one per budget, pointing at the budget, with what
// PDB_DISRUPTION_BLOCKED would have shown (numbers as last reported) and the
// condition.
func syncFailedFinding(ns, name string, healthy, desired, allowed int32, message string, nodes []string, pods ...findings.PDBPod) findings.Finding {
	return findings.Finding{
		RuleID:   "PDB_SYNC_FAILED",
		Severity: findings.Unknown,
		Resource: findings.ResourceRef{Kind: "PodDisruptionBudget", Namespace: ns, Name: name},
		Evidence: findings.PDBSyncFailedEvidence{
			PDBEvidence: findings.PDBEvidence{
				Namespace:          ns,
				Name:               name,
				CurrentHealthy:     healthy,
				DesiredHealthy:     desired,
				DisruptionsAllowed: allowed,
				Nodes:              nodes,
				Pods:               pods,
			},
			Reason:  "SyncFailed",
			Message: message,
		},
	}
}

func TestPDBSyncFailed(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-sync-failed.yaml")
	apiPDB := func(nodes []string, pods ...findings.PDBPod) findings.Finding {
		return syncFailedFinding("billing", "api-pdb", 2, 2, 0, "found no controllers for pod api-0", nodes, pods...)
	}
	webPDB := syncFailedFinding("shop", "web-pdb", 2, 1, 1, "scale subresource not found", []string{"worker-01"}, pdbPod("shop", "web-0", "worker-01"))

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "failed budgets are reported whatever they allow; settled, stale, DaemonSet-only and off-target budgets are not",
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
			name:    "a failed budget is reported when its pod is on a target node",
			targets: []string{"worker-03"},
			want: []findings.Finding{
				syncFailedFinding("edge", "proxy-pdb", 1, 1, 0, "scale subresource not found", []string{"worker-03"}, pdbPod("edge", "proxy-0", "worker-03")),
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
			got := Run(nil, Input{Snapshot: s, Targets: tt.targets}, PDBSyncFailed{})

			// Ignore prose; it is covered by TestPDBSyncFailedDescribesFinding.
			for i := range got {
				got[i].Description, got[i].Remediation = "", ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

// A budget the controller failed on must show up once, as UNKNOWN:
// PDB_DISRUPTION_BLOCKED steps aside for it, a budget that is stale as well is
// left to PDB_STALE_STATUS, and a settled budget that allows nothing stays
// BLOCKED. The rules share the pod selection, so no budget falls between them.
func TestPDBSyncFailedReplacesBlocked(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-sync-failed.yaml")

	got := Run(nil, Input{Snapshot: s, Targets: []string{"worker-01", "worker-02"}}, PDBDisruptionBlocked{}, PDBStaleStatus{}, PDBSyncFailed{})

	var summary []string
	for _, f := range got {
		summary = append(summary, fmt.Sprintf("%s %s %s", f.Severity, f.RuleID, f.Resource))
	}
	want := []string{
		"BLOCKED PDB_DISRUPTION_BLOCKED PodDisruptionBudget/batch/job-pdb",
		"UNKNOWN PDB_SYNC_FAILED PodDisruptionBudget/billing/api-pdb",
		"UNKNOWN PDB_STALE_STATUS PodDisruptionBudget/legacy/old-pdb",
		"UNKNOWN PDB_SYNC_FAILED PodDisruptionBudget/shop/web-pdb",
	}
	if !slices.Equal(summary, want) {
		t.Errorf("findings\n got: %q\nwant: %q", summary, want)
	}
}

func TestPDBSyncFailedDescribesFinding(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-sync-failed.yaml")

	tests := []struct {
		name    string
		targets []string
		want    string
	}{
		{
			name:    "states the controller error and the pod and node counts",
			targets: []string{"worker-01", "worker-02"},
			want:    "The disruption controller could not compute PodDisruptionBudget billing/api-pdb (found no controllers for pod api-0), so the numbers in its status cannot be trusted and draincheck cannot tell whether eviction of its 2 pods on 2 target nodes will be refused.",
		},
		{
			name:    "singular for one pod on one node",
			targets: []string{"worker-02"},
			want:    "The disruption controller could not compute PodDisruptionBudget billing/api-pdb (found no controllers for pod api-0), so the numbers in its status cannot be trusted and draincheck cannot tell whether eviction of its 1 pod on 1 target node will be refused.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *findings.Finding
			for _, f := range (PDBSyncFailed{}).Evaluate(Input{Snapshot: s, Targets: tt.targets}) {
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

func TestPDBSyncFailure(t *testing.T) {
	cond := func(typ, reason string) metav1.Condition {
		return metav1.Condition{Type: typ, Reason: reason, Message: "m"}
	}
	tests := []struct {
		name  string
		conds []metav1.Condition
		want  bool
	}{
		{"no conditions", nil, false},
		{"sync failed", []metav1.Condition{cond(policyv1.DisruptionAllowedCondition, policyv1.SyncFailedReason)}, true},
		{"sufficient pods", []metav1.Condition{cond(policyv1.DisruptionAllowedCondition, policyv1.SufficientPodsReason)}, false},
		{"insufficient pods is a plain budget at 0", []metav1.Condition{cond(policyv1.DisruptionAllowedCondition, policyv1.InsufficientPodsReason)}, false},
		{"the reason on another condition type does not count", []metav1.Condition{cond("Other", policyv1.SyncFailedReason)}, false},
		{"found among other conditions", []metav1.Condition{cond("Other", "X"), cond(policyv1.DisruptionAllowedCondition, policyv1.SyncFailedReason)}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdb := policyv1.PodDisruptionBudget{Status: policyv1.PodDisruptionBudgetStatus{Conditions: tt.conds}}
			if _, got := pdbSyncFailure(pdb); got != tt.want {
				t.Errorf("pdbSyncFailure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPDBSyncFailedNilSnapshot(t *testing.T) {
	if got := (PDBSyncFailed{}).Evaluate(Input{Targets: []string{"worker-01"}}); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestPDBSyncFailedID(t *testing.T) {
	if got := (PDBSyncFailed{}).ID(); got != "PDB_SYNC_FAILED" {
		t.Errorf("ID() = %q", got)
	}
}
