package analyzer

import (
	"testing"
	"time"

	"github.com/Veinar/draincheck/internal/inventory"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSkippedByDrain(t *testing.T) {
	controller, notController := true, false
	ref := func(kind, name string, isController *bool) []metav1.OwnerReference {
		return []metav1.OwnerReference{{Kind: kind, Name: name, Controller: isController}}
	}
	replicaSet := appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
		Namespace:       "shop",
		Name:            "web-7f9c",
		OwnerReferences: ref("Deployment", "web", &controller),
	}}
	owners := inventory.NewOwnerResolver(inventory.NewSnapshot(nil, nil, []appsv1.ReplicaSet{replicaSet}, nil, time.Time{}))

	tests := []struct {
		name string
		meta metav1.ObjectMeta
		want bool
	}{
		{
			name: "pod with no controller is evicted",
			meta: metav1.ObjectMeta{Namespace: "shop", Name: "debug"},
			want: false,
		},
		{
			name: "pod of a Deployment is evicted",
			meta: metav1.ObjectMeta{Namespace: "shop", Name: "web-7f9c-a", OwnerReferences: ref("ReplicaSet", "web-7f9c", &controller)},
			want: false,
		},
		{
			name: "pod of a StatefulSet is evicted",
			meta: metav1.ObjectMeta{Namespace: "shop", Name: "db-0", OwnerReferences: ref("StatefulSet", "db", &controller)},
			want: false,
		},
		{
			name: "pod of a DaemonSet is never evicted",
			meta: metav1.ObjectMeta{Namespace: "shop", Name: "agent-x1", OwnerReferences: ref("DaemonSet", "agent", &controller)},
			want: true,
		},
		{
			name: "a DaemonSet reference that is not the controller does not count",
			meta: metav1.ObjectMeta{Namespace: "shop", Name: "odd-0", OwnerReferences: ref("DaemonSet", "agent", &notController)},
			want: false,
		},
		{
			name: "static pod is never evicted",
			meta: metav1.ObjectMeta{Namespace: "kube-system", Name: "etcd-worker-01", Annotations: map[string]string{mirrorPodAnnotation: "abc"}},
			want: true,
		},
		{
			name: "static pod is never evicted although its owner is a Node",
			meta: metav1.ObjectMeta{
				Namespace:       "kube-system",
				Name:            "etcd-worker-01",
				Annotations:     map[string]string{mirrorPodAnnotation: "abc"},
				OwnerReferences: ref("Node", "worker-01", &controller),
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := corev1.Pod{ObjectMeta: tt.meta}
			if got := skippedByDrain(pod, owners); got != tt.want {
				t.Errorf("skippedByDrain() = %v, want %v", got, tt.want)
			}
		})
	}
}
