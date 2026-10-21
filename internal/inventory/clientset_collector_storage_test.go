package inventory

import (
	"context"
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestClientsetCollectorCollectsStorage(t *testing.T) {
	client := fake.NewClientset(
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data-db-0", Namespace: "billing"}},
		&corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "cache", Namespace: "shop"}},
		&corev1.PersistentVolume{ObjectMeta: metav1.ObjectMeta{Name: "pv-0001"}},
	)

	s, err := NewClientsetCollector(client, nil).Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if got := len(s.PersistentVolumeClaims()); got != 2 {
		t.Errorf("pvcs = %d, want 2 (all namespaces)", got)
	}
	if got := len(s.PersistentVolumes()); got != 1 {
		t.Errorf("pvs = %d, want 1", got)
	}
}

func TestClientsetCollectorStorageListError(t *testing.T) {
	for _, resource := range []string{"persistentvolumeclaims", "persistentvolumes"} {
		t.Run(resource, func(t *testing.T) {
			client := fake.NewClientset()
			client.PrependReactor("list", resource, func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, errors.New("forbidden")
			})
			s, err := NewClientsetCollector(client, nil).Collect(context.Background())
			if err == nil {
				t.Fatal("expected an error, an empty list would hide findings")
			}
			if s != nil {
				t.Errorf("snapshot = %v, want nil on error", s)
			}
		})
	}
}

func TestClientsetCollectorReadsStorageWithListOnly(t *testing.T) {
	client := fake.NewClientset()
	if _, err := NewClientsetCollector(client, nil).Collect(context.Background()); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, a := range client.Actions() {
		seen[a.GetResource().Resource] = true
		if a.GetVerb() != "list" {
			t.Errorf("non-read call: %s %s", a.GetVerb(), a.GetResource().Resource)
		}
	}
	for _, resource := range []string{"persistentvolumeclaims", "persistentvolumes"} {
		if !seen[resource] {
			t.Errorf("%s was never listed", resource)
		}
	}
}
