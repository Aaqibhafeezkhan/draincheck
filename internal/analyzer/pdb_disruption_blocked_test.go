package analyzer

import (
	"fmt"
	"reflect"
	"slices"
	"testing"
	"time"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// pdbPod is a pod entry in PDB evidence.
func pdbPod(ns, name, node string) findings.PDBPod {
	return findings.PDBPod{Namespace: ns, Name: name, Node: node}
}

// pdbFinding is the finding for the budget ns/name: one per budget, pointing at
// the budget, with the nodes and pods it selects in evidence.
func pdbFinding(ns, name string, healthy, desired int32, nodes []string, pods ...findings.PDBPod) findings.Finding {
	return findings.Finding{
		RuleID:   "PDB_DISRUPTION_BLOCKED",
		Severity: findings.Blocked,
		Resource: findings.ResourceRef{Kind: "PodDisruptionBudget", Namespace: ns, Name: name},
		Evidence: findings.PDBEvidence{
			Namespace:      ns,
			Name:           name,
			CurrentHealthy: healthy,
			DesiredHealthy: desired,
			Nodes:          nodes,
			Pods:           pods,
		},
	}
}

func TestPDBDisruptionBlocked(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-disruption-blocked.yaml")

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "one finding per budget; finished, DaemonSet and static pods, allowed budgets and unusable selectors are not reported",
			targets: []string{"worker-01"},
			want: []findings.Finding{
				pdbFinding("batch", "all-pods", 1, 1, []string{"worker-01"}, pdbPod("batch", "job-0", "worker-01")),
				pdbFinding("billing", "api-pdb", 3, 3, []string{"worker-01"}, pdbPod("billing", "api-0", "worker-01")),
				pdbFinding("shop", "cache-pdb-a", 1, 1, []string{"worker-01"}, pdbPod("shop", "cache-0", "worker-01")),
				pdbFinding("shop", "cache-pdb-b", 1, 1, []string{"worker-01"}, pdbPod("shop", "cache-0", "worker-01")),
			},
		},
		{
			name:    "only pods on the target nodes count",
			targets: []string{"worker-02"},
			want: []findings.Finding{
				pdbFinding("billing", "api-pdb", 3, 3, []string{"worker-02"}, pdbPod("billing", "api-1", "worker-02")),
			},
		},
		{
			name:    "a budget whose pods are on three target nodes is one finding that lists all three nodes and pods",
			targets: []string{"worker-01", "worker-02", "worker-03"},
			want: []findings.Finding{
				pdbFinding("batch", "all-pods", 1, 1, []string{"worker-01"}, pdbPod("batch", "job-0", "worker-01")),
				pdbFinding("billing", "api-pdb", 3, 3, []string{"worker-01", "worker-02", "worker-03"},
					pdbPod("billing", "api-0", "worker-01"),
					pdbPod("billing", "api-1", "worker-02"),
					pdbPod("billing", "api-2", "worker-03")),
				pdbFinding("shop", "cache-pdb-a", 1, 1, []string{"worker-01"}, pdbPod("shop", "cache-0", "worker-01")),
				pdbFinding("shop", "cache-pdb-b", 1, 1, []string{"worker-01"}, pdbPod("shop", "cache-0", "worker-01")),
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

// A budget whose status has not caught up with its spec reports numbers that
// cannot be trusted, so it is not a BLOCKED finding; PDB_STALE_STATUS reports it
// as UNKNOWN instead. A settled budget that allows nothing stays BLOCKED.
func TestPDBDisruptionBlockedSkipsStalePDB(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-stale-status.yaml")

	got := PDBDisruptionBlocked{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01", "worker-02", "worker-03"}})

	if len(got) != 1 || got[0].Resource.Name != "job-pdb" {
		t.Fatalf("got %+v, want only the settled batch/job-pdb", got)
	}
}

// ten replicas behind one budget are the case the grouping exists for: one
// finding, with all ten pods in evidence, however the snapshot orders them.
func TestPDBDisruptionBlockedGroupsReplicas(t *testing.T) {
	labels := map[string]string{"app": "api"}
	var pods []corev1.Pod
	for i := 9; i >= 0; i-- { // reverse order, so the rule has to sort
		pods = append(pods, corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "billing", Name: fmt.Sprintf("api-%d", i), Labels: labels},
			Spec:       corev1.PodSpec{NodeName: fmt.Sprintf("worker-%02d", i%4)},
		})
	}
	pdb := policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Namespace: "billing", Name: "api-pdb"},
		Spec:       policyv1.PodDisruptionBudgetSpec{Selector: &metav1.LabelSelector{MatchLabels: labels}},
		Status:     policyv1.PodDisruptionBudgetStatus{CurrentHealthy: 10, DesiredHealthy: 10},
	}
	s := inventory.NewSnapshot(nil, pods, nil, []policyv1.PodDisruptionBudget{pdb}, time.Time{})
	targets := []string{"worker-00", "worker-01", "worker-02", "worker-03"}

	got := PDBDisruptionBlocked{}.Evaluate(Input{Snapshot: s, Targets: targets})

	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(got), got)
	}
	ev, ok := got[0].Evidence.(findings.PDBEvidence)
	if !ok {
		t.Fatalf("evidence is %T, want findings.PDBEvidence", got[0].Evidence)
	}
	if len(ev.Pods) != 10 {
		t.Fatalf("evidence lists %d pods, want 10: %+v", len(ev.Pods), ev.Pods)
	}
	for i, p := range ev.Pods {
		if want := fmt.Sprintf("api-%d", i); p.Name != want || p.Namespace != "billing" {
			t.Errorf("pods[%d] = %s/%s, want billing/%s", i, p.Namespace, p.Name, want)
		}
	}
	if !slices.Equal(ev.Nodes, targets) {
		t.Errorf("nodes = %v, want %v", ev.Nodes, targets)
	}
}

func TestPDBDisruptionBlockedDescribesFinding(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-disruption-blocked.yaml")

	tests := []struct {
		name    string
		targets []string
		want    string
	}{
		{
			name:    "states the numbers once and the pod and node counts",
			targets: []string{"worker-01", "worker-02", "worker-03"},
			want:    "PodDisruptionBudget billing/api-pdb allows 0 disruptions (3 healthy, 3 desired), so eviction of its 3 pods on 3 target nodes will be refused.",
		},
		{
			name:    "singular for one pod on one node",
			targets: []string{"worker-02"},
			want:    "PodDisruptionBudget billing/api-pdb allows 0 disruptions (3 healthy, 3 desired), so eviction of its 1 pod on 1 target node will be refused.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *findings.Finding
			for _, f := range (PDBDisruptionBlocked{}).Evaluate(Input{Snapshot: s, Targets: tt.targets}) {
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

func TestPDBDisruptionBlockedNilSnapshot(t *testing.T) {
	if got := (PDBDisruptionBlocked{}).Evaluate(Input{Targets: []string{"worker-01"}}); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestPDBDisruptionBlockedID(t *testing.T) {
	if got := (PDBDisruptionBlocked{}).ID(); got != "PDB_DISRUPTION_BLOCKED" {
		t.Errorf("ID() = %q", got)
	}
}
