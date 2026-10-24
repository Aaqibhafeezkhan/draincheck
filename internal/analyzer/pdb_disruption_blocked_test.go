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
			name:    "one finding per budget; finished, DaemonSet and static pods, allowed budgets, overlapping budgets and unusable selectors are not reported",
			targets: []string{"worker-01"},
			want: []findings.Finding{
				pdbFinding("batch", "all-pods", 1, 1, []string{"worker-01"}, pdbPod("batch", "job-0", "worker-01")),
				pdbFinding("billing", "api-pdb", 3, 3, []string{"worker-01"}, pdbPod("billing", "api-0", "worker-01")),
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
			got := Run(nil, Input{Snapshot: s, Targets: tt.targets}, PDBDisruptionBlocked{})

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
			Status:     podWithReady(corev1.ConditionTrue).Status, // a pod with no Ready condition is not ready
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
			want:    "PodDisruptionBudget billing/api-pdb allows 0 disruptions (3 healthy, 3 desired), so eviction of its 3 pods on 3 target nodes will be refused. kubectl drain retries a refused eviction until --timeout, which is infinite by default, so it waits and does not complete.",
		},
		{
			name:    "singular for one pod on one node",
			targets: []string{"worker-02"},
			want:    "PodDisruptionBudget billing/api-pdb allows 0 disruptions (3 healthy, 3 desired), so eviction of its 1 pod on 1 target node will be refused. kubectl drain retries a refused eviction until --timeout, which is infinite by default, so it waits and does not complete.",
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

// The eviction API lets a pod that is not Ready past its budget (see
// evictedPastBudget) and never consults a budget for a Pending or terminating
// pod. A budget at 0 is BLOCKED only for the pods it can refuse.
func TestPDBDisruptionBlockedUnhealthyPods(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-unhealthy-pods.yaml")
	targets := []string{"worker-01", "worker-02", "worker-03"}

	mix := pdbFinding("mix", "mix-pdb", 2, 2, []string{"worker-01"}, pdbPod("mix", "mix-0", "worker-01"), pdbPod("mix", "mix-1", "worker-01"))
	mixEv := mix.Evidence.(findings.PDBEvidence)
	mixEv.EvictableUnhealthyPods = []findings.PDBPod{pdbPod("mix", "mix-2", "worker-01")}
	mix.Evidence = mixEv

	// An unhealthy budget still refuses an unhealthy pod under IfHealthyBudget.
	sick := pdbFinding("sick", "sick-pdb", 1, 2, []string{"worker-01"}, pdbPod("sick", "sick-0", "worker-01"))

	// A policy value draincheck does not know: nothing is guessed.
	odd := pdbFinding("odd", "odd-pdb", 1, 1, []string{"worker-01"}, pdbPod("odd", "odd-0", "worker-01"))
	odd.Severity = findings.Unknown
	oddEv := odd.Evidence.(findings.PDBEvidence)
	oddEv.UnhealthyPodEvictionPolicy = "SometimesAllow"
	odd.Evidence = oddEv

	got := Run(nil, Input{Snapshot: s, Targets: targets}, PDBDisruptionBlocked{})
	for i := range got {
		got[i].Description, got[i].Remediation = "", ""
	}
	// solo, aa and bypass select only pods the budget cannot refuse; stale is
	// left to PDB_STALE_STATUS; cnt and cnt2 allow disruptions.
	want := []findings.Finding{mix, odd, sick}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, want)
	}
}

func TestPDBDisruptionBlockedDescribesUnhealthyPods(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-unhealthy-pods.yaml")
	targets := []string{"worker-01", "worker-02", "worker-03"}

	tests := []struct {
		name string
		pdb  string
		want string
	}{
		{
			name: "an unhealthy budget says it refuses unhealthy pods too",
			pdb:  "sick-pdb",
			want: "PodDisruptionBudget sick/sick-pdb allows 0 disruptions (1 healthy, 2 desired), so eviction of its 1 pod on 1 target node will be refused. kubectl drain retries a refused eviction until --timeout, which is infinite by default, so it waits and does not complete. The budget itself is unhealthy, so with IfHealthyBudget it refuses the pods that are not Ready as well.",
		},
		{
			name: "an unknown policy says why the answer is not known",
			pdb:  "odd-pdb",
			want: "PodDisruptionBudget odd/odd-pdb allows 0 disruptions (1 healthy, 1 desired), but its unhealthyPodEvictionPolicy \"SometimesAllow\" is not one draincheck knows, so it cannot tell whether eviction of its 1 pod on 1 target node will be refused.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *findings.Finding
			for _, f := range (PDBDisruptionBlocked{}).Evaluate(Input{Snapshot: s, Targets: targets}) {
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
		})
	}
}

// Budgets that select the same evicted pod keep the numbers of whichever sync
// last picked them (#139), so a 0 or a spare disruption on one of them says
// nothing about the drain. PDB_OVERLAPPING_SELECTORS reports the pod; this rule
// does not also report the budget. A budget that is not overlapped is
// unaffected.
func TestPDBDisruptionBlockedSkipsOverlappingPDBs(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-overlapping-numbers.yaml")

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "dup-b is at 0 but overlaps dup-a; cart-pdb is at 0 alone and is reported",
			targets: []string{"worker-01", "worker-02"},
			want: []findings.Finding{
				pdbFinding("shop", "cart-pdb", 2, 2, []string{"worker-01", "worker-02"},
					pdbPod("shop", "cart-0", "worker-01"), pdbPod("shop", "cart-1", "worker-02")),
			},
		},
		{
			name:    "side-a is at 0; it overlaps side-b only for a pod off the target, so it is reported",
			targets: []string{"worker-02"},
			want: []findings.Finding{
				pdbFinding("shop", "cart-pdb", 2, 2, []string{"worker-02"}, pdbPod("shop", "cart-1", "worker-02")),
				pdbFinding("shop", "side-a", 2, 1, []string{"worker-02"}, pdbPod("shop", "side-1", "worker-02")),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Run(nil, Input{Snapshot: s, Targets: tt.targets}, PDBDisruptionBlocked{})
			for i := range got {
				got[i].Description, got[i].Remediation = "", ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}
