package inventory

import (
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func workloadsFixture() Workloads {
	return Workloads{
		Deployments:  []appsv1.Deployment{{ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "billing", Labels: map[string]string{"app": "api"}}}},
		StatefulSets: []appsv1.StatefulSet{{ObjectMeta: metav1.ObjectMeta{Name: "db", Namespace: "billing", Labels: map[string]string{"app": "db"}}}},
		Jobs:         []batchv1.Job{{ObjectMeta: metav1.ObjectMeta{Name: "report-29", Namespace: "billing", Labels: map[string]string{"app": "report"}}}},
		CronJobs:     []batchv1.CronJob{{ObjectMeta: metav1.ObjectMeta{Name: "report", Namespace: "billing", Labels: map[string]string{"app": "report"}}}},
	}
}

func TestSnapshotWorkloadsEmptyByDefault(t *testing.T) {
	s := NewSnapshot(nil, nil, nil, nil, time.Time{})

	if len(s.Deployments()) != 0 || len(s.StatefulSets()) != 0 || len(s.Jobs()) != 0 || len(s.CronJobs()) != 0 {
		t.Errorf("a snapshot built without WithWorkloads should hold no workloads")
	}
}

func TestSnapshotExposesWorkloads(t *testing.T) {
	s := NewSnapshot(nil, nil, nil, nil, time.Time{}, WithWorkloads(workloadsFixture()))

	if got := s.Deployments(); len(got) != 1 || got[0].Name != "api" {
		t.Errorf("Deployments() = %+v", got)
	}
	if got := s.StatefulSets(); len(got) != 1 || got[0].Name != "db" {
		t.Errorf("StatefulSets() = %+v", got)
	}
	if got := s.Jobs(); len(got) != 1 || got[0].Name != "report-29" {
		t.Errorf("Jobs() = %+v", got)
	}
	if got := s.CronJobs(); len(got) != 1 || got[0].Name != "report" {
		t.Errorf("CronJobs() = %+v", got)
	}
}

func TestSnapshotWorkloadsAreIsolatedFromInput(t *testing.T) {
	w := workloadsFixture()
	s := NewSnapshot(nil, nil, nil, nil, time.Time{}, WithWorkloads(w))

	w.Deployments[0].Labels["app"] = "changed"
	w.StatefulSets[0].Labels["app"] = "changed"
	w.Jobs[0].Labels["app"] = "changed"
	w.CronJobs[0].Labels["app"] = "changed"

	if got := s.Deployments()[0].Labels["app"]; got != "api" {
		t.Errorf("deployment label = %q, input mutation leaked into the snapshot", got)
	}
	if got := s.StatefulSets()[0].Labels["app"]; got != "db" {
		t.Errorf("statefulset label = %q, input mutation leaked into the snapshot", got)
	}
	if got := s.Jobs()[0].Labels["app"]; got != "report" {
		t.Errorf("job label = %q, input mutation leaked into the snapshot", got)
	}
	if got := s.CronJobs()[0].Labels["app"]; got != "report" {
		t.Errorf("cronjob label = %q, input mutation leaked into the snapshot", got)
	}
}

func TestSnapshotWorkloadsAreIsolatedFromCallers(t *testing.T) {
	s := NewSnapshot(nil, nil, nil, nil, time.Time{}, WithWorkloads(workloadsFixture()))

	s.Deployments()[0].Labels["app"] = "changed"
	s.StatefulSets()[0].Labels["app"] = "changed"
	s.Jobs()[0].Labels["app"] = "changed"
	s.CronJobs()[0].Labels["app"] = "changed"

	if got := s.Deployments()[0].Labels["app"]; got != "api" {
		t.Errorf("deployment label = %q, a caller changed the snapshot", got)
	}
	if got := s.StatefulSets()[0].Labels["app"]; got != "db" {
		t.Errorf("statefulset label = %q, a caller changed the snapshot", got)
	}
	if got := s.Jobs()[0].Labels["app"]; got != "report" {
		t.Errorf("job label = %q, a caller changed the snapshot", got)
	}
	if got := s.CronJobs()[0].Labels["app"]; got != "report" {
		t.Errorf("cronjob label = %q, a caller changed the snapshot", got)
	}
}
