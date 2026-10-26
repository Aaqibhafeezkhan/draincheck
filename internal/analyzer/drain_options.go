package analyzer

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// DrainOptions are the kubectl drain flags the operator will pass. Without a
// flag, draincheck judges a pod as kubectl drain without it would.
type DrainOptions struct {
	// IgnoreDaemonSets is --ignore-daemonsets.
	IgnoreDaemonSets bool
	// DeleteEmptyDirData is --delete-emptydir-data.
	DeleteEmptyDirData bool
	// Force is --force.
	Force bool
	// DisableEviction is --disable-eviction.
	DisableEviction bool
	// PodSelector is --pod-selector: the drain acts only on pods whose labels
	// match. Nil selects every pod.
	PodSelector labels.Selector
}

// selects reports whether --pod-selector lets the drain act on pod. kubectl
// lists only the pods that match, so the others are never touched.
func (o DrainOptions) selects(pod corev1.Pod) bool {
	return o.PodSelector == nil || o.PodSelector.Matches(labels.Set(pod.Labels))
}
