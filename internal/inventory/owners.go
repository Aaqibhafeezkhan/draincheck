package inventory

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Owner is the top-level controller of a pod. The zero Owner means the pod has
// no controller (a "naked" pod).
type Owner struct {
	Kind string
	// Namespace is the pod's namespace; controllers are namespaced like their pods.
	Namespace string
	Name      string
}

// Unmanaged reports whether the pod has no controller.
func (o Owner) Unmanaged() bool { return o.Kind == "" }

// OwnerResolver maps pods to their top-level controller using a Snapshot.
type OwnerResolver struct {
	replicaSets map[replicaSetKey]metav1.OwnerReference
}

type replicaSetKey struct{ namespace, name string }

// NewOwnerResolver indexes the ReplicaSets of s.
func NewOwnerResolver(s *Snapshot) *OwnerResolver {
	r := &OwnerResolver{replicaSets: make(map[replicaSetKey]metav1.OwnerReference)}
	for i := range s.rss {
		rs := &s.rss[i]
		if ref := controllerRef(rs.OwnerReferences); ref != nil {
			r.replicaSets[replicaSetKey{rs.Namespace, rs.Name}] = *ref
		}
	}
	return r
}

// Resolve returns the top-level controller of pod.
//
// A ReplicaSet resolves to the controller of that ReplicaSet (normally a
// Deployment). A ReplicaSet that is missing from the snapshot, or has no
// controller, is reported as the ReplicaSet itself. DaemonSet, StatefulSet,
// Job and any other kind are reported as given. Only references marked as
// controller are considered.
func (r *OwnerResolver) Resolve(pod corev1.Pod) Owner {
	ref := controllerRef(pod.OwnerReferences)
	if ref == nil {
		return Owner{}
	}
	if ref.Kind == "ReplicaSet" {
		if parent, ok := r.replicaSets[replicaSetKey{pod.Namespace, ref.Name}]; ok {
			return Owner{Kind: parent.Kind, Namespace: pod.Namespace, Name: parent.Name}
		}
	}
	return Owner{Kind: ref.Kind, Namespace: pod.Namespace, Name: ref.Name}
}

// controllerRef returns the reference with Controller=true, or nil.
func controllerRef(refs []metav1.OwnerReference) *metav1.OwnerReference {
	for i := range refs {
		if refs[i].Controller != nil && *refs[i].Controller {
			return &refs[i]
		}
	}
	return nil
}
