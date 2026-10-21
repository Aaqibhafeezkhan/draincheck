package inventory

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func storageFixture() Storage {
	return Storage{
		PersistentVolumeClaims: []corev1.PersistentVolumeClaim{{ObjectMeta: metav1.ObjectMeta{Name: "data-db-0", Namespace: "billing", Labels: map[string]string{"app": "db"}}}},
		PersistentVolumes:      []corev1.PersistentVolume{{ObjectMeta: metav1.ObjectMeta{Name: "pv-0001", Labels: map[string]string{"disk": "local"}}}},
	}
}

func TestSnapshotStorageEmptyByDefault(t *testing.T) {
	s := NewSnapshot(nil, nil, nil, nil, time.Time{})

	if len(s.PersistentVolumeClaims()) != 0 || len(s.PersistentVolumes()) != 0 {
		t.Errorf("a snapshot built without WithStorage should hold no volumes")
	}
}

func TestSnapshotExposesStorage(t *testing.T) {
	s := NewSnapshot(nil, nil, nil, nil, time.Time{}, WithStorage(storageFixture()))

	if got := s.PersistentVolumeClaims(); len(got) != 1 || got[0].Name != "data-db-0" {
		t.Errorf("PersistentVolumeClaims() = %+v", got)
	}
	if got := s.PersistentVolumes(); len(got) != 1 || got[0].Name != "pv-0001" {
		t.Errorf("PersistentVolumes() = %+v", got)
	}
}

func TestSnapshotStorageIsIsolatedFromInput(t *testing.T) {
	st := storageFixture()
	s := NewSnapshot(nil, nil, nil, nil, time.Time{}, WithStorage(st))

	st.PersistentVolumeClaims[0].Labels["app"] = "changed"
	st.PersistentVolumes[0].Labels["disk"] = "changed"

	if got := s.PersistentVolumeClaims()[0].Labels["app"]; got != "db" {
		t.Errorf("pvc label = %q, input mutation leaked into the snapshot", got)
	}
	if got := s.PersistentVolumes()[0].Labels["disk"]; got != "local" {
		t.Errorf("pv label = %q, input mutation leaked into the snapshot", got)
	}
}

func TestSnapshotStorageIsIsolatedFromCallers(t *testing.T) {
	s := NewSnapshot(nil, nil, nil, nil, time.Time{}, WithStorage(storageFixture()))

	s.PersistentVolumeClaims()[0].Labels["app"] = "changed"
	s.PersistentVolumes()[0].Labels["disk"] = "changed"

	if got := s.PersistentVolumeClaims()[0].Labels["app"]; got != "db" {
		t.Errorf("pvc label = %q, a caller changed the snapshot", got)
	}
	if got := s.PersistentVolumes()[0].Labels["disk"]; got != "local" {
		t.Errorf("pv label = %q, a caller changed the snapshot", got)
	}
}
