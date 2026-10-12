package inventory

import (
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func controlledBy(kind, name string) []metav1.OwnerReference {
	yes := true
	return []metav1.OwnerReference{{Kind: kind, Name: name, Controller: &yes}}
}

func podOwnedBy(refs []metav1.OwnerReference) corev1.Pod {
	return corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "p", Namespace: "shop", OwnerReferences: refs}}
}

func TestResolveOwner(t *testing.T) {
	rss := []appsv1.ReplicaSet{
		{ObjectMeta: metav1.ObjectMeta{Name: "web-7d9", Namespace: "shop", OwnerReferences: controlledBy("Deployment", "web")}},
		{ObjectMeta: metav1.ObjectMeta{Name: "standalone", Namespace: "shop"}},
		// Same name as the one above, different namespace and different owner.
		{ObjectMeta: metav1.ObjectMeta{Name: "web-7d9", Namespace: "other", OwnerReferences: controlledBy("Deployment", "elsewhere")}},
	}
	r := NewOwnerResolver(NewSnapshot(nil, nil, rss, nil, time.Time{}))

	tests := []struct {
		name string
		pod  corev1.Pod
		want Owner
	}{
		{"replicaset resolves to deployment", podOwnedBy(controlledBy("ReplicaSet", "web-7d9")), Owner{Kind: "Deployment", Namespace: "shop", Name: "web"}},
		{"replicaset without owner stays replicaset", podOwnedBy(controlledBy("ReplicaSet", "standalone")), Owner{Kind: "ReplicaSet", Namespace: "shop", Name: "standalone"}},
		{"replicaset missing from snapshot stays replicaset", podOwnedBy(controlledBy("ReplicaSet", "gone")), Owner{Kind: "ReplicaSet", Namespace: "shop", Name: "gone"}},
		{"daemonset", podOwnedBy(controlledBy("DaemonSet", "agent")), Owner{Kind: "DaemonSet", Namespace: "shop", Name: "agent"}},
		{"statefulset", podOwnedBy(controlledBy("StatefulSet", "db")), Owner{Kind: "StatefulSet", Namespace: "shop", Name: "db"}},
		{"job", podOwnedBy(controlledBy("Job", "migrate")), Owner{Kind: "Job", Namespace: "shop", Name: "migrate"}},
		{"unknown kind is reported as is", podOwnedBy(controlledBy("Rollout", "web")), Owner{Kind: "Rollout", Namespace: "shop", Name: "web"}},
		{"no owner is unmanaged", podOwnedBy(nil), Owner{}},
		{
			"non-controller reference is ignored",
			podOwnedBy([]metav1.OwnerReference{{Kind: "ReplicaSet", Name: "web-7d9"}}),
			Owner{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := r.Resolve(tt.pod); got != tt.want {
				t.Errorf("Resolve() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestOwnerUnmanaged(t *testing.T) {
	if !(Owner{}).Unmanaged() {
		t.Error("zero Owner should be unmanaged")
	}
	if (Owner{Kind: "Job", Name: "x"}).Unmanaged() {
		t.Error("Owner with a kind should not be unmanaged")
	}
}
