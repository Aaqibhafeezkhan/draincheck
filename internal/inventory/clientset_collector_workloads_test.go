package inventory

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestClientsetCollectorCollectsWorkloads(t *testing.T) {
	client := fake.NewClientset(
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "billing"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "shop"}},
		&appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "billing"}},
		&batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: "report-29", Namespace: "billing"}},
		&batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "report", Namespace: "billing"}},
		&batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: "cleanup", Namespace: "shop"}},
	)

	s, err := NewClientsetCollector(client, nil).Collect(context.Background())
	if err != nil {
		t.Fatalf("Collect() error = %v", err)
	}
	if got := len(s.Deployments()); got != 2 {
		t.Errorf("deployments = %d, want 2 (all namespaces)", got)
	}
	if got := len(s.StatefulSets()); got != 1 {
		t.Errorf("statefulsets = %d, want 1", got)
	}
	if got := len(s.Jobs()); got != 1 {
		t.Errorf("jobs = %d, want 1", got)
	}
	if got := len(s.CronJobs()); got != 2 {
		t.Errorf("cronjobs = %d, want 2", got)
	}
}

func TestClientsetCollectorWorkloadListError(t *testing.T) {
	for _, resource := range []string{"deployments", "statefulsets", "jobs", "cronjobs"} {
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

func TestClientsetCollectorReadsWorkloadsWithListOnly(t *testing.T) {
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
	for _, resource := range []string{"deployments", "statefulsets", "jobs", "cronjobs"} {
		if !seen[resource] {
			t.Errorf("%s was never listed", resource)
		}
	}
}
