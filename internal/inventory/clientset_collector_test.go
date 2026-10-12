package inventory

import (
	"context"
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var _ Collector = (*ClientsetCollector)(nil)

func TestClientsetCollectorCollect(t *testing.T) {
	client := fake.NewClientset(
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-01"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-0", Namespace: "billing"}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-0", Namespace: "shop"}},
		&appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Name: "api-7d9", Namespace: "billing"}},
		&policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "billing"}},
	)
	now := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	c := NewClientsetCollector(client)
	c.now = func() time.Time { return now }

	s, err := c.Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if got := len(s.Nodes()); got != 1 {
		t.Errorf("nodes = %d, want 1", got)
	}
	if got := len(s.Pods()); got != 2 {
		t.Errorf("pods = %d, want 2 (all namespaces)", got)
	}
	if got := len(s.ReplicaSets()); got != 1 {
		t.Errorf("replicasets = %d, want 1", got)
	}
	if got := len(s.PodDisruptionBudgets()); got != 1 {
		t.Errorf("pdbs = %d, want 1", got)
	}
	if !s.Timestamp().Equal(now) {
		t.Errorf("timestamp = %v, want %v", s.Timestamp(), now)
	}
}

func TestClientsetCollectorIsReadOnly(t *testing.T) {
	client := fake.NewClientset()
	if _, err := NewClientsetCollector(client).Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	actions := client.Actions()
	if len(actions) == 0 {
		t.Fatal("no API calls recorded")
	}
	for _, a := range actions {
		if a.GetVerb() != "list" {
			t.Errorf("non-read call: %s %s", a.GetVerb(), a.GetResource().Resource)
		}
	}
}

func TestClientsetCollectorListError(t *testing.T) {
	for _, resource := range []string{"nodes", "pods", "replicasets", "poddisruptionbudgets"} {
		t.Run(resource, func(t *testing.T) {
			client := fake.NewClientset()
			client.PrependReactor("list", resource, func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("boom")
			})
			_, err := NewClientsetCollector(client).Collect(context.Background())
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
