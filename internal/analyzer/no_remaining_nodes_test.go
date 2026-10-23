package analyzer

import (
	"encoding/json"
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

// podsNamed is the snapshot of no-remaining-nodes-taints.yaml with only the
// named pods left, so each case below moves exactly the pods it is about.
func podsNamed(t *testing.T, names ...string) *inventory.Snapshot {
	t.Helper()
	s := snapshotFromFixture(t, "no-remaining-nodes-taints.yaml")
	var pods []corev1.Pod
	for _, p := range s.Pods() {
		if slices.Contains(names, p.Name) {
			pods = append(pods, p)
		}
	}
	return inventory.NewSnapshot(s.Nodes(), pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())
}

// The commonest small cluster: the nodes left are Ready and not cordoned, but
// a taint keeps the moving pods off them.
func TestNoRemainingNodesTaints(t *testing.T) {
	cpTaint := findings.NodeTaint{Key: "node-role.kubernetes.io/control-plane", Effect: "NoSchedule"}
	gpuTaint := findings.NodeTaint{Key: "dedicated", Effect: "NoExecute"}

	tests := []struct {
		name    string
		pods    []string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "no moving pod tolerates the taints of the nodes left",
			pods:    []string{"web-0"},
			targets: []string{"worker", "soft"},
			want: []findings.Finding{{
				RuleID:   "NO_REMAINING_NODES",
				Severity: findings.Risk,
				Resource: findings.ResourceRef{Kind: "Node", Name: "soft,worker"},
				Evidence: findings.NoRemainingNodesEvidence{
					PodsToMove: 1, TargetNodes: 2, TotalNodes: 4, RemainingSchedulable: 0,
					TaintedNodes: []findings.TaintedNode{
						{Name: "control-plane", Taints: []findings.NodeTaint{cpTaint}},
						{Name: "gpu", Taints: []findings.NodeTaint{gpuTaint}},
					},
				},
			}},
		},
		{
			name:    "a pod that tolerates the control-plane taint can land there",
			pods:    []string{"web-0", "web-1"},
			targets: []string{"worker", "soft"},
		},
		{
			name:    "a NoExecute taint is tolerated by Equal with the same value",
			pods:    []string{"gpu-0"},
			targets: []string{"worker", "soft"},
		},
		{
			// Room for some pods and none for others is a per-workload finding
			// that this rule does not make; it stays silent.
			name:    "room for some of the moving pods is not reported",
			pods:    []string{"web-0", "gpu-0"},
			targets: []string{"worker", "soft"},
		},
		{
			name:    "PreferNoSchedule never removes room",
			pods:    []string{"web-0"},
			targets: []string{"worker"},
		},
		{
			name:    "a taint changes nothing when no pod moves",
			pods:    nil,
			targets: []string{"worker", "soft"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NoRemainingNodes{}.Evaluate(Input{Snapshot: podsNamed(t, tt.pods...), Targets: tt.targets})
			if !reflect.DeepEqual(got, withText(tt.want, got)) {
				t.Fatalf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

// A node that is cordoned is named by NODE_NOT_SCHEDULABLE; this rule lists
// only the nodes a taint alone rules out.
func TestNoRemainingNodesTaintsSkipCordonedNodes(t *testing.T) {
	cp := nodeWith("control-plane", false, corev1.ConditionTrue)
	cp.Spec.Taints = []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}}
	cordoned := nodeWith("old", true, corev1.ConditionTrue)
	cordoned.Spec.Taints = cp.Spec.Taints
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-0", Namespace: "shop", OwnerReferences: []metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "StatefulSet", Name: "web", Controller: ptr.To(true)}}},
		Spec:   corev1.PodSpec{NodeName: "worker"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning}}
	s := inventory.NewSnapshot([]corev1.Node{nodeWith("worker", false, corev1.ConditionTrue), cp, cordoned}, []corev1.Pod{pod}, nil, nil, time.Time{})

	got := NoRemainingNodes{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker"}})
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %+v", got)
	}
	ev, ok := got[0].Evidence.(findings.NoRemainingNodesEvidence)
	if !ok || len(ev.TaintedNodes) != 1 || ev.TaintedNodes[0].Name != "control-plane" {
		t.Errorf("want only control-plane listed as tainted, got %+v", got[0].Evidence)
	}
}

func TestNoRemainingNodesTaintsText(t *testing.T) {
	got := NoRemainingNodes{}.Evaluate(Input{Snapshot: podsNamed(t, "web-0"), Targets: []string{"worker", "soft"}})
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d", len(got))
	}
	for _, want := range []string{"taints that none of the moving pods tolerates", "control-plane", "gpu"} {
		if !strings.Contains(got[0].Description, want) {
			t.Errorf("description %q lacks %q", got[0].Description, want)
		}
	}
	if !strings.Contains(got[0].Remediation, "kubectl describe node") {
		t.Errorf("remediation %q lacks the taint check", got[0].Remediation)
	}
}

// A pressure taint is a taint like any other: the node is Ready and not
// cordoned, yet a pod that tolerates nothing cannot land on it.
func TestNoRemainingNodesPressureTaint(t *testing.T) {
	pressure := taintedNode("left", corev1.Taint{Key: "node.kubernetes.io/memory-pressure", Effect: corev1.TaintEffectNoSchedule})
	pod := ownedPod("web-0", "shop", "StatefulSet", "web", "worker", defaultTolerations...)
	s := inventory.NewSnapshot([]corev1.Node{nodeWith("worker", false, corev1.ConditionTrue), pressure}, []corev1.Pod{pod}, nil, nil, time.Time{})

	got := NoRemainingNodes{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker"}})
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %+v", got)
	}
	ev := got[0].Evidence.(findings.NoRemainingNodesEvidence)
	want := []findings.TaintedNode{{Name: "left", Taints: []findings.NodeTaint{{Key: "node.kubernetes.io/memory-pressure", Effect: "NoSchedule"}}}}
	if !reflect.DeepEqual(ev.TaintedNodes, want) {
		t.Errorf("tainted nodes = %+v, want %+v", ev.TaintedNodes, want)
	}
}

// The tolerations the API server adds to every pod must not count as room on a
// tainted control plane.
func TestNoRemainingNodesDefaultTolerationsAreNotRoom(t *testing.T) {
	cp := taintedNode("cp", corev1.Taint{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule})
	pod := ownedPod("web-0", "shop", "StatefulSet", "web", "worker", defaultTolerations...)
	s := inventory.NewSnapshot([]corev1.Node{nodeWith("worker", false, corev1.ConditionTrue), cp}, []corev1.Pod{pod}, nil, nil, time.Time{})
	if got := (NoRemainingNodes{}).Evaluate(Input{Snapshot: s, Targets: []string{"worker"}}); len(got) != 1 {
		t.Errorf("want 1 finding, got %+v", got)
	}
}

// A pod that is already terminating has its replacement already, so it is not
// a pod that needs room, even when no node would take it.
func TestNoRemainingNodesTerminatingPodNeedsNoRoom(t *testing.T) {
	cp := taintedNode("cp", corev1.Taint{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule})
	pod := ownedPod("web-0", "shop", "StatefulSet", "web", "worker")
	pod.DeletionTimestamp = &metav1.Time{Time: time.Unix(1, 0)}
	s := inventory.NewSnapshot([]corev1.Node{nodeWith("worker", false, corev1.ConditionTrue), cp}, []corev1.Pod{pod}, nil, nil, time.Time{})
	if got := (NoRemainingNodes{}).Evaluate(Input{Snapshot: s, Targets: []string{"worker"}}); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

// Targets and nodes in any order give the same tainted nodes, sorted by name,
// and the taints of a node stay in the order the node lists them.
func TestNoRemainingNodesTaintsIgnoreOrder(t *testing.T) {
	s := podsNamed(t, "web-0")
	targets := []string{"worker", "soft"}
	want := NoRemainingNodes{}.Evaluate(Input{Snapshot: s, Targets: targets})

	nodes, pods := s.Nodes(), s.Pods()
	slices.Reverse(nodes)
	slices.Reverse(pods)
	reversed := inventory.NewSnapshot(nodes, pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())
	got := NoRemainingNodes{}.Evaluate(Input{Snapshot: reversed, Targets: []string{"soft", "worker"}})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order changed the result\n got: %+v\nwant: %+v", got, want)
	}
	ev := got[0].Evidence.(findings.NoRemainingNodesEvidence)
	if len(ev.TaintedNodes) != 2 || ev.TaintedNodes[0].Name != "control-plane" || ev.TaintedNodes[1].Name != "gpu" {
		t.Errorf("tainted nodes not sorted by name: %+v", ev.TaintedNodes)
	}
}

// A list in the evidence is never null in the JSON.
func TestNoRemainingNodesTaintsJSONHasNoNull(t *testing.T) {
	got := NoRemainingNodes{}.Evaluate(Input{Snapshot: podsNamed(t, "web-0"), Targets: []string{"worker", "soft"}})
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Errorf("the JSON has a null: %s", b)
	}
}
