package inventory

import (
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func fixture() ([]corev1.Node, []corev1.Pod, []appsv1.ReplicaSet, []policyv1.PodDisruptionBudget, time.Time) {
	nodes := []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "worker-01"}}}
	pods := []corev1.Pod{{ObjectMeta: metav1.ObjectMeta{Name: "api-0", Namespace: "billing", Labels: map[string]string{"app": "api"}}}}
	rss := []appsv1.ReplicaSet{{ObjectMeta: metav1.ObjectMeta{Name: "api-7d9", Namespace: "billing", Labels: map[string]string{"app": "api"}}}}
	pdbs := []policyv1.PodDisruptionBudget{{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "billing"}}}
	return nodes, pods, rss, pdbs, time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
}

func TestNewSnapshotExposesInput(t *testing.T) {
	nodes, pods, rss, pdbs, ts := fixture()
	s := NewSnapshot(nodes, pods, rss, pdbs, ts)

	if got := s.Nodes(); len(got) != 1 || got[0].Name != "worker-01" {
		t.Errorf("Nodes() = %+v", got)
	}
	if got := s.Pods(); len(got) != 1 || got[0].Name != "api-0" {
		t.Errorf("Pods() = %+v", got)
	}
	if got := s.ReplicaSets(); len(got) != 1 || got[0].Name != "api-7d9" {
		t.Errorf("ReplicaSets() = %+v", got)
	}
	if got := s.PodDisruptionBudgets(); len(got) != 1 || got[0].Name != "api" {
		t.Errorf("PodDisruptionBudgets() = %+v", got)
	}
	if !s.Timestamp().Equal(ts) {
		t.Errorf("Timestamp() = %v, want %v", s.Timestamp(), ts)
	}
}

func TestSnapshotIsIsolatedFromInput(t *testing.T) {
	nodes, pods, rss, pdbs, ts := fixture()
	s := NewSnapshot(nodes, pods, rss, pdbs, ts)

	nodes[0].Name = "changed"
	pods[0].Labels["app"] = "changed"
	rss[0].Labels["app"] = "changed"
	pdbs[0].Name = "changed"

	if got := s.ReplicaSets()[0].Labels["app"]; got != "api" {
		t.Errorf("replicaset labels leaked input mutation: %q", got)
	}
	if got := s.Nodes()[0].Name; got != "worker-01" {
		t.Errorf("node name leaked input mutation: %q", got)
	}
	if got := s.Pods()[0].Labels["app"]; got != "api" {
		t.Errorf("pod labels leaked input mutation: %q", got)
	}
	if got := s.PodDisruptionBudgets()[0].Name; got != "api" {
		t.Errorf("pdb name leaked input mutation: %q", got)
	}
}

func TestSnapshotIsIsolatedFromAccessors(t *testing.T) {
	nodes, pods, rss, pdbs, ts := fixture()
	s := NewSnapshot(nodes, pods, rss, pdbs, ts)

	s.Nodes()[0].Name = "changed"
	s.Pods()[0].Labels["app"] = "changed"
	s.ReplicaSets()[0].Labels["app"] = "changed"
	s.PodDisruptionBudgets()[0].Name = "changed"

	if got := s.ReplicaSets()[0].Labels["app"]; got != "api" {
		t.Errorf("replicaset labels mutated via accessor: %q", got)
	}
	if got := s.Nodes()[0].Name; got != "worker-01" {
		t.Errorf("node name mutated via accessor: %q", got)
	}
	if got := s.Pods()[0].Labels["app"]; got != "api" {
		t.Errorf("pod labels mutated via accessor: %q", got)
	}
	if got := s.PodDisruptionBudgets()[0].Name; got != "api" {
		t.Errorf("pdb name mutated via accessor: %q", got)
	}
}
