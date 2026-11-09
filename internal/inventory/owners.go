package inventory

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Owner is the top-level controller of a pod. The zero Owner means the pod has
// no controller (a "naked" pod).
type Owner struct {
	// APIGroup is the API group of Kind: "apps" for a Deployment, "batch" for a
	// Job, "" for the core group, and the operator's own group for a custom
	// resource.
	APIGroup string
	Kind     string
	// Namespace is the pod's namespace; controllers are namespaced like their pods.
	Namespace string
	Name      string
}

// Unmanaged reports whether the pod has no controller.
func (o Owner) Unmanaged() bool { return o.Kind == "" }

// OwnerResolver maps pods to their top-level controller using a Snapshot.
type OwnerResolver struct {
	replicaSets map[replicaSetKey]metav1.OwnerReference
	// daemonSets holds the DaemonSets of the snapshot; daemonSetsKnown is false
	// when the snapshot never collected them, so none can be called gone.
	daemonSets      map[replicaSetKey]bool
	daemonSetsKnown bool
}

type replicaSetKey struct{ namespace, name string }

// NewOwnerResolver indexes the ReplicaSets of s.
func NewOwnerResolver(s *Snapshot) *OwnerResolver {
	r := &OwnerResolver{
		replicaSets:     make(map[replicaSetKey]metav1.OwnerReference),
		daemonSets:      make(map[replicaSetKey]bool),
		daemonSetsKnown: s.workloadsLoaded,
	}
	for i := range s.daemonSets {
		r.daemonSets[replicaSetKey{s.daemonSets[i].Namespace, s.daemonSets[i].Name}] = true
	}
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
			return Owner{APIGroup: apiGroupOf(parent.APIVersion), Kind: parent.Kind, Namespace: pod.Namespace, Name: parent.Name}
		}
	}
	return Owner{APIGroup: apiGroupOf(ref.APIVersion), Kind: ref.Kind, Namespace: pod.Namespace, Name: ref.Name}
}

// DaemonSetGone reports whether pod is controlled by an apps/v1 DaemonSet that is
// not in the snapshot: the DaemonSet was deleted and the pod outlived it. It is
// false when the snapshot never collected DaemonSets, so no pod is called an
// orphan on a guess. A nil resolver knows nothing, so it is false too.
func (r *OwnerResolver) DaemonSetGone(pod corev1.Pod) bool {
	ref := controllerRef(pod.OwnerReferences)
	if r == nil || !r.daemonSetsKnown || ref == nil || ref.Kind != "DaemonSet" || apiGroupOf(ref.APIVersion) != "apps" {
		return false
	}
	return !r.daemonSets[replicaSetKey{pod.Namespace, ref.Name}]
}

// apiGroupOf returns the API group of an owner reference's apiVersion
// ("group/version", or just "version" for the core group). A value that is not
// a valid group/version is returned whole, so it is never taken for a
// built-in group.
func apiGroupOf(apiVersion string) string {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return apiVersion
	}
	return gv.Group
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
