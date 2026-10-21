package analyzer

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// allNodes is every node of node-not-schedulable.yaml.
var allNodes = []string{"worker-01", "worker-02", "worker-03", "spare-1", "spare-2", "spare-3", "spare-4", "spare-5"}

// noRemainingFinding is the one finding for a drain that leaves no node able to
// take the evicted pods. resource is the sorted target names, joined by commas.
func noRemainingFinding(resource string, podsToMove, targets, total int) findings.Finding {
	return findings.Finding{
		RuleID:   "NO_REMAINING_NODES",
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: "Node", Name: resource},
		Evidence: findings.NoRemainingNodesEvidence{PodsToMove: podsToMove, TargetNodes: targets, TotalNodes: total, RemainingSchedulable: 0},
	}
}

func TestNoRemainingNodes(t *testing.T) {
	s := snapshotFromFixture(t, "node-not-schedulable.yaml")

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "every node is a target: the pods that move have nowhere to go",
			targets: allNodes,
			// worker-01 (2) + worker-03 (1) move; 8 nodes in the snapshot.
			want: []findings.Finding{noRemainingFinding(strings.Join(slices.Sorted(slices.Values(allNodes)), ","), 3, 8, 8)},
		},
		{
			name:    "the nodes left are all cordoned or not Ready",
			targets: []string{"worker-01", "worker-02", "spare-4", "spare-5"},
			// Left: worker-03 (cordoned), spare-1 (cordoned), spare-2 and spare-3 (not Ready).
			want: []findings.Finding{noRemainingFinding("spare-4,spare-5,worker-01,worker-02", 2, 4, 8)},
		},
		{
			name:    "a node that reports no Ready condition still counts as room",
			targets: []string{"worker-01", "worker-02", "spare-4"},
		},
		{
			name:    "a schedulable node outside the targets is room",
			targets: []string{"worker-01"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NoRemainingNodes{}.Evaluate(Input{Snapshot: s, Targets: tt.targets})
			if !reflect.DeepEqual(got, withText(tt.want, got)) {
				t.Fatalf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

func TestNoRemainingNodesText(t *testing.T) {
	s := snapshotFromFixture(t, "node-not-schedulable.yaml")
	got := NoRemainingNodes{}.Evaluate(Input{Snapshot: s, Targets: allNodes})
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d", len(got))
	}
	for _, want := range []string{"8 of 8 nodes", "no node is left", "3 pods", "Pending"} {
		if !strings.Contains(got[0].Description, want) {
			t.Errorf("description %q lacks %q", got[0].Description, want)
		}
	}
	if got[0].Remediation == "" {
		t.Error("remediation is empty")
	}
}

func TestNoRemainingNodesNilSnapshot(t *testing.T) {
	if got := (NoRemainingNodes{}).Evaluate(Input{Targets: allNodes}); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

func TestNoRemainingNodesID(t *testing.T) {
	if got := (NoRemainingNodes{}).ID(); got != "NO_REMAINING_NODES" {
		t.Errorf("ID() = %q", got)
	}
}

// Draining the only node is nothing to report when no pod needs a new home.
func TestNoRemainingNodesNothingToMove(t *testing.T) {
	daemon := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "agent-x1", Namespace: "monitoring", OwnerReferences: []metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "DaemonSet", Name: "agent", Controller: ptr.To(true)}}},
		Spec: corev1.PodSpec{NodeName: "a"}}
	s := inventory.NewSnapshot([]corev1.Node{nodeWith("a", false, corev1.ConditionTrue)}, []corev1.Pod{daemon}, nil, nil, time.Time{})
	if got := (NoRemainingNodes{}).Evaluate(Input{Snapshot: s, Targets: []string{"a"}}); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

// The result must not depend on the order of the targets, nodes or pods.
func TestNoRemainingNodesIgnoresOrder(t *testing.T) {
	s := snapshotFromFixture(t, "node-not-schedulable.yaml")
	want := NoRemainingNodes{}.Evaluate(Input{Snapshot: s, Targets: allNodes})

	nodes, pods := s.Nodes(), s.Pods()
	slices.Reverse(nodes)
	slices.Reverse(pods)
	reversed := inventory.NewSnapshot(nodes, pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())
	targets := slices.Clone(allNodes)
	slices.Reverse(targets)
	got := NoRemainingNodes{}.Evaluate(Input{Snapshot: reversed, Targets: targets})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order changed the result\n got: %+v\nwant: %+v", got, want)
	}
}
