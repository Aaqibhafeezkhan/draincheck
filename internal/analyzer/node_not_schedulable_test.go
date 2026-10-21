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

// notSchedulableFinding is the one finding for the nodes that stay up but take
// no evicted pods. remaining counts every node outside the target set.
func notSchedulableFinding(resource string, podsToMove, remaining int, nodes ...findings.UnschedulableNode) findings.Finding {
	return findings.Finding{
		RuleID:   "NODE_NOT_SCHEDULABLE",
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: "Node", Name: resource},
		Evidence: findings.NodeNotSchedulableEvidence{PodsToMove: podsToMove, RemainingNodes: remaining, Nodes: nodes},
	}
}

var (
	spare1   = findings.UnschedulableNode{Name: "spare-1", Cordoned: true, ReadyStatus: "True"}
	spare2   = findings.UnschedulableNode{Name: "spare-2", ReadyStatus: "False"}
	spare3   = findings.UnschedulableNode{Name: "spare-3", ReadyStatus: "Unknown"}
	worker03 = findings.UnschedulableNode{Name: "worker-03", Cordoned: true, ReadyStatus: "True"}
)

func TestNodeNotSchedulable(t *testing.T) {
	s := snapshotFromFixture(t, "node-not-schedulable.yaml")

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "pods that move cannot land on cordoned or not Ready nodes; a node with no Ready condition is not flagged",
			targets: []string{"worker-01"},
			// Remaining: worker-02, worker-03, spare-1..5 = 7.
			want: []findings.Finding{notSchedulableFinding("spare-1,spare-2,spare-3,worker-03", 2, 7, spare1, spare2, spare3, worker03)},
		},
		{
			name:    "a target that is already cordoned is not reported, but its pods still need room",
			targets: []string{"worker-01", "worker-03"},
			// Remaining: worker-02, spare-1..5 = 6.
			want: []findings.Finding{notSchedulableFinding("spare-1,spare-2,spare-3", 3, 6, spare1, spare2, spare3)},
		},
		{
			name:    "nothing is reported when no pod needs a new home (DaemonSet, static, terminal and controller-less pods)",
			targets: []string{"worker-02"},
		},
		{
			name:    "nothing is reported when every cordoned or not Ready node is itself a target",
			targets: []string{"worker-01", "worker-03", "spare-1", "spare-2", "spare-3"},
		},
		{
			name:    "nothing is reported when no pod runs on the targets",
			targets: []string{"spare-4"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NodeNotSchedulable{}.Evaluate(Input{Snapshot: s, Targets: tt.targets})
			if !reflect.DeepEqual(got, withText(tt.want, got)) {
				t.Fatalf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

// withText copies Description and Remediation from got into want, so the table
// states the typed fields and TestNodeNotSchedulableText states the wording.
func withText(want, got []findings.Finding) []findings.Finding {
	if len(want) == 0 || len(want) != len(got) {
		return want
	}
	out := make([]findings.Finding, len(want))
	for i := range want {
		out[i] = want[i]
		out[i].Description = got[i].Description
		out[i].Remediation = got[i].Remediation
	}
	return out
}

func TestNodeNotSchedulableText(t *testing.T) {
	s := snapshotFromFixture(t, "node-not-schedulable.yaml")
	got := NodeNotSchedulable{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}})
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d", len(got))
	}
	for _, want := range []string{
		"4 of the 7 nodes that stay up cannot take evicted pods",
		"spare-1 (cordoned)", "spare-2 (not Ready)", "spare-3 (not Ready)", "worker-03 (cordoned)",
		"2 pods",
		"land on at most 3 nodes",
	} {
		if !strings.Contains(got[0].Description, want) {
			t.Errorf("description %q lacks %q", got[0].Description, want)
		}
	}
	if got[0].Remediation == "" {
		t.Error("remediation is empty")
	}
}

func TestNodeNotSchedulableNilSnapshot(t *testing.T) {
	if got := (NodeNotSchedulable{}).Evaluate(Input{Targets: []string{"worker-01"}}); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

func TestNodeNotSchedulableID(t *testing.T) {
	if got := (NodeNotSchedulable{}).ID(); got != "NODE_NOT_SCHEDULABLE" {
		t.Errorf("ID() = %q", got)
	}
}

func TestNodeStateWords(t *testing.T) {
	tests := []struct {
		node findings.UnschedulableNode
		want string
	}{
		{findings.UnschedulableNode{Cordoned: true, ReadyStatus: "True"}, "cordoned"},
		{findings.UnschedulableNode{Cordoned: true}, "cordoned"},
		{findings.UnschedulableNode{ReadyStatus: "False"}, "not Ready"},
		{findings.UnschedulableNode{ReadyStatus: "Unknown"}, "not Ready"},
		{findings.UnschedulableNode{Cordoned: true, ReadyStatus: "False"}, "cordoned and not Ready"},
		{findings.UnschedulableNode{Cordoned: true, ReadyStatus: "Unknown"}, "cordoned and not Ready"},
	}
	for _, tt := range tests {
		if got := nodeStateWords(tt.node); got != tt.want {
			t.Errorf("nodeStateWords(%+v) = %q, want %q", tt.node, got, tt.want)
		}
	}
}

// The result must not depend on the order of the nodes or pods in the snapshot.
func TestNodeNotSchedulableIgnoresOrder(t *testing.T) {
	s := snapshotFromFixture(t, "node-not-schedulable.yaml")
	in := Input{Snapshot: s, Targets: []string{"worker-01"}}
	want := NodeNotSchedulable{}.Evaluate(in)

	nodes, pods := s.Nodes(), s.Pods()
	slices.Reverse(nodes)
	slices.Reverse(pods)
	reversed := inventory.NewSnapshot(nodes, pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())
	got := NodeNotSchedulable{}.Evaluate(Input{Snapshot: reversed, Targets: in.Targets})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order changed the result\n got: %+v\nwant: %+v", got, want)
	}
}

// A node that is cordoned and not Ready is one entry, with both facts.
func TestNodeNotSchedulableCordonedAndNotReady(t *testing.T) {
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "ns", OwnerReferences: []metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "StatefulSet", Name: "db", Controller: ptr.To(true)}}},
		Spec: corev1.PodSpec{NodeName: "a"}}
	s := inventory.NewSnapshot(
		[]corev1.Node{nodeWith("a", false, corev1.ConditionTrue), nodeWith("b", true, corev1.ConditionFalse)},
		[]corev1.Pod{pod}, nil, nil, time.Time{})
	got := NodeNotSchedulable{}.Evaluate(Input{Snapshot: s, Targets: []string{"a"}})
	want := []findings.UnschedulableNode{{Name: "b", Cordoned: true, ReadyStatus: "False"}}
	if len(got) != 1 || !reflect.DeepEqual(got[0].Evidence.(findings.NodeNotSchedulableEvidence).Nodes, want) {
		t.Fatalf("got %+v", got)
	}
	if !strings.Contains(got[0].Description, "b (cordoned and not Ready)") || !strings.Contains(got[0].Description, "land on at most 0 nodes") {
		t.Errorf("description %q", got[0].Description)
	}
}
