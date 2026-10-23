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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

// onlyPods is the snapshot of workload-untolerated-taints.yaml with only the named pods
// left, so each case moves exactly the pods it is about.
func onlyPods(t *testing.T, names ...string) *inventory.Snapshot {
	t.Helper()
	s := snapshotFromFixture(t, "workload-untolerated-taints.yaml")
	var pods []corev1.Pod
	for _, p := range s.Pods() {
		if slices.Contains(names, p.Name) {
			pods = append(pods, p)
		}
	}
	return inventory.NewSnapshot(s.Nodes(), pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())
}

func dbUntolerated() findings.Finding {
	return findings.Finding{
		RuleID:   "WORKLOAD_UNTOLERATED_TAINTS",
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: "StatefulSet", Namespace: "shop", Name: "db"},
		Evidence: findings.WorkloadUntoleratedTaintsEvidence{
			OwnerKind: "StatefulSet", OwnerName: "db",
			Pods:           []findings.StrandedPod{{Name: "db-0", Node: "worker"}, {Name: "db-1", Node: "worker"}},
			RemainingNodes: 2,
			TaintedNodes: []findings.TaintedNode{
				{Name: "control-plane", Taints: []findings.NodeTaint{{Key: "node-role.kubernetes.io/control-plane", Effect: "NoSchedule"}}},
				{Name: "gpu", Taints: []findings.NodeTaint{{Key: "dedicated", Effect: "NoExecute"}}},
			},
		},
	}
}

func TestWorkloadUntoleratedTaints(t *testing.T) {
	both := []string{"worker", "soft"}
	tests := []struct {
		name    string
		pods    []string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "a workload that tolerates nothing is stranded while others can land",
			pods:    []string{"db-0", "db-1", "proxy-0", "trainer-0", "agent-x1"},
			targets: both,
			want:    []findings.Finding{dbUntolerated()},
		},
		{
			name:    "a PreferNoSchedule node is room",
			pods:    []string{"db-0", "db-1", "proxy-0"},
			targets: []string{"worker"},
		},
		{
			// NO_REMAINING_NODES states this case for the whole run.
			name:    "every moving pod is stranded: left to NO_REMAINING_NODES",
			pods:    []string{"db-0", "db-1"},
			targets: both,
		},
		{
			// NO_REMAINING_NODES and NODE_NOT_SCHEDULABLE cover a run with no node left.
			name:    "no node is left at all",
			pods:    []string{"db-0", "proxy-0"},
			targets: []string{"worker", "soft", "control-plane", "gpu"},
		},
		{
			name:    "a DaemonSet pod does not move, so it is not stranded",
			pods:    []string{"agent-x1", "proxy-0"},
			targets: both,
		},
		{
			name:    "nothing moves",
			pods:    nil,
			targets: both,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := WorkloadUntoleratedTaints{}.Evaluate(Input{Snapshot: onlyPods(t, tt.pods...), Targets: tt.targets})
			if !reflect.DeepEqual(got, withText(tt.want, got)) {
				t.Fatalf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

// Two stranded workloads give two findings, in a stable order.
func TestWorkloadUntoleratedTaintsOneFindingPerWorkload(t *testing.T) {
	s := snapshotFromFixture(t, "workload-untolerated-taints.yaml")
	// Give proxy-0 no toleration, so db and proxy are both stranded while
	// trainer (which tolerates gpu) is not.
	var pods []corev1.Pod
	for _, p := range s.Pods() {
		if p.Name == "proxy-0" {
			p.Spec.Tolerations = nil
		}
		pods = append(pods, p)
	}
	s = inventory.NewSnapshot(s.Nodes(), pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())

	got := WorkloadUntoleratedTaints{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker", "soft"}})
	var names []string
	for _, f := range got {
		names = append(names, f.Resource.Name)
	}
	if !slices.Equal(names, []string{"db", "proxy"}) {
		t.Errorf("workloads = %v, want [db proxy]", names)
	}
}

func TestWorkloadUntoleratedTaintsText(t *testing.T) {
	got := WorkloadUntoleratedTaints{}.Evaluate(Input{Snapshot: snapshotFromFixture(t, "workload-untolerated-taints.yaml"), Targets: []string{"worker", "soft"}})
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d", len(got))
	}
	for _, want := range []string{"StatefulSet shop/db", "2 pods", "control-plane", "gpu", "Pending"} {
		if !strings.Contains(got[0].Description, want) {
			t.Errorf("description %q lacks %q", got[0].Description, want)
		}
	}
	if !strings.Contains(got[0].Remediation, "kubectl describe node") {
		t.Errorf("remediation %q lacks the taint check", got[0].Remediation)
	}
}

func TestWorkloadUntoleratedTaintsNilSnapshot(t *testing.T) {
	if got := (WorkloadUntoleratedTaints{}).Evaluate(Input{Targets: []string{"worker"}}); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

func TestWorkloadUntoleratedTaintsID(t *testing.T) {
	if got := (WorkloadUntoleratedTaints{}).ID(); got != "WORKLOAD_UNTOLERATED_TAINTS" {
		t.Errorf("ID() = %q", got)
	}
}

// The result must not depend on the order of the targets, nodes or pods.
func TestWorkloadUntoleratedTaintsIgnoresOrder(t *testing.T) {
	s := snapshotFromFixture(t, "workload-untolerated-taints.yaml")
	targets := []string{"worker", "soft"}
	want := WorkloadUntoleratedTaints{}.Evaluate(Input{Snapshot: s, Targets: targets})

	nodes, pods := s.Nodes(), s.Pods()
	slices.Reverse(nodes)
	slices.Reverse(pods)
	reversed := inventory.NewSnapshot(nodes, pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())
	got := WorkloadUntoleratedTaints{}.Evaluate(Input{Snapshot: reversed, Targets: []string{"soft", "worker"}})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order changed the result\n got: %+v\nwant: %+v", got, want)
	}
}

// clusterWithProxy has a worker (the target), a tainted control plane, and a
// proxy pod that tolerates the taint, so a run is never wholly stranded and the
// workload under test is judged on its own.
func clusterWithProxy(pods []corev1.Pod, rss []appsv1.ReplicaSet) *inventory.Snapshot {
	cp := taintedNode("cp", corev1.Taint{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule})
	proxy := ownedPod("proxy-0", "shop", "StatefulSet", "proxy", "worker",
		append([]corev1.Toleration{{Key: "node-role.kubernetes.io/control-plane", Operator: corev1.TolerationOpExists}}, defaultTolerations...)...)
	return inventory.NewSnapshot([]corev1.Node{nodeWith("worker", false, corev1.ConditionTrue), cp},
		append([]corev1.Pod{proxy}, pods...), rss, nil, time.Time{})
}

func replicaSet(name, namespace, deployment string) appsv1.ReplicaSet {
	return appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, OwnerReferences: []metav1.OwnerReference{
		{APIVersion: "apps/v1", Kind: "Deployment", Name: deployment, Controller: ptr.To(true)}}}}
}

func TestWorkloadUntoleratedTaintsResolvesTheOwner(t *testing.T) {
	// The pods of a Deployment are owned by its ReplicaSet; the finding names the Deployment.
	web := ownedPod("web-abc-1", "shop", "ReplicaSet", "web-abc", "worker", defaultTolerations...)
	s := clusterWithProxy([]corev1.Pod{web}, []appsv1.ReplicaSet{replicaSet("web-abc", "shop", "web")})
	got := WorkloadUntoleratedTaints{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker"}})
	if len(got) != 1 || got[0].Resource != (findings.ResourceRef{Kind: "Deployment", Namespace: "shop", Name: "web"}) {
		t.Fatalf("want one finding for Deployment shop/web, got %+v", got)
	}

	// A ReplicaSet missing from the snapshot is its own workload.
	orphan := clusterWithProxy([]corev1.Pod{web}, nil)
	got = WorkloadUntoleratedTaints{}.Evaluate(Input{Snapshot: orphan, Targets: []string{"worker"}})
	if len(got) != 1 || got[0].Resource.Kind != "ReplicaSet" || got[0].Resource.Name != "web-abc" {
		t.Fatalf("want one finding for ReplicaSet web-abc, got %+v", got)
	}
}

// The same workload name in two namespaces is two workloads.
func TestWorkloadUntoleratedTaintsSameNameInTwoNamespaces(t *testing.T) {
	a := ownedPod("db-0", "team-a", "StatefulSet", "db", "worker")
	b := ownedPod("db-0", "team-b", "StatefulSet", "db", "worker")
	got := WorkloadUntoleratedTaints{}.Evaluate(Input{Snapshot: clusterWithProxy([]corev1.Pod{b, a}, nil), Targets: []string{"worker"}})
	var namespaces []string
	for _, f := range got {
		namespaces = append(namespaces, f.Resource.Namespace)
	}
	if !slices.Equal(namespaces, []string{"team-a", "team-b"}) {
		t.Errorf("namespaces = %v, want [team-a team-b]", namespaces)
	}
}

// During a rollout a Deployment has two ReplicaSets whose pods tolerate
// different things. If any of its pods can land, the workload is not reported.
func TestWorkloadUntoleratedTaintsRolloutWithDifferentTolerations(t *testing.T) {
	oldPod := ownedPod("web-old-1", "shop", "ReplicaSet", "web-old", "worker")
	newPod := ownedPod("web-new-1", "shop", "ReplicaSet", "web-new", "worker",
		corev1.Toleration{Key: "node-role.kubernetes.io/control-plane", Operator: corev1.TolerationOpExists})
	s := clusterWithProxy([]corev1.Pod{oldPod, newPod}, []appsv1.ReplicaSet{replicaSet("web-old", "shop", "web"), replicaSet("web-new", "shop", "web")})
	if got := (WorkloadUntoleratedTaints{}).Evaluate(Input{Snapshot: s, Targets: []string{"worker"}}); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

// A pod that is already terminating is not one that needs room.
func TestWorkloadUntoleratedTaintsIgnoresTerminatingPods(t *testing.T) {
	db := ownedPod("db-0", "shop", "StatefulSet", "db", "worker")
	db.DeletionTimestamp = &metav1.Time{Time: time.Unix(1, 0)}
	if got := (WorkloadUntoleratedTaints{}).Evaluate(Input{Snapshot: clusterWithProxy([]corev1.Pod{db}, nil), Targets: []string{"worker"}}); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

// The tolerations every pod gets rule out nothing on a control plane, and the
// JSON has no null.
func TestWorkloadUntoleratedTaintsDefaultTolerationsAndJSON(t *testing.T) {
	db := ownedPod("db-0", "shop", "StatefulSet", "db", "worker", defaultTolerations...)
	got := WorkloadUntoleratedTaints{}.Evaluate(Input{Snapshot: clusterWithProxy([]corev1.Pod{db}, nil), Targets: []string{"worker"}})
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %+v", got)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Errorf("the JSON has a null: %s", b)
	}
}

// A target that is not in the snapshot, or no target at all, moves nothing.
func TestWorkloadUntoleratedTaintsUnknownOrNoTargets(t *testing.T) {
	db := ownedPod("db-0", "shop", "StatefulSet", "db", "worker")
	s := clusterWithProxy([]corev1.Pod{db}, nil)
	for _, targets := range [][]string{nil, {"ghost"}} {
		if got := (WorkloadUntoleratedTaints{}).Evaluate(Input{Snapshot: s, Targets: targets}); got != nil {
			t.Errorf("targets %v: want nil, got %+v", targets, got)
		}
	}
}
