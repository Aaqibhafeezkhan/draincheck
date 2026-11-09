package analyzer

import (
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// drainAction is what kubectl drain does with a pod after its filters ran.
type drainAction int

const (
	// drainDelete: the pod is evicted (or deleted).
	drainDelete drainAction = iota
	// drainSkip: the pod stays on the node and the drain goes on.
	drainSkip
	// drainRefuse: the drain refuses to start; no pod is evicted.
	drainRefuse
)

// drainReason is the filter that decided, when it has something to say.
type drainReason int

const (
	reasonNone drainReason = iota
	// reasonDaemonSet: a DaemonSet-managed pod.
	reasonDaemonSet
	// reasonDaemonSetGone: a pod whose DaemonSet no longer exists. With
	// drainDelete it is the warning kubectl prints for --force.
	reasonDaemonSetGone
	// reasonMirror: a static (mirror) pod.
	reasonMirror
	// reasonLocalStorage: a pod with an emptyDir volume.
	reasonLocalStorage
	// reasonUnmanaged: a pod with no controller. With drainDelete it is the
	// warning kubectl prints for --force.
	reasonUnmanaged
)

// drainVerdict is the answer of the filters for one pod.
type drainVerdict struct {
	action drainAction
	reason drainReason
}

// drainFilter is one kubectl pod filter.
// owners tells the DaemonSet filter whether a pod's DaemonSet still exists; it may be nil.
type drainFilter func(pod corev1.Pod, o DrainOptions, owners *inventory.OwnerResolver) drainVerdict

// drainFilters are kubectl's pod filters in the order it applies them
// (k8s.io/kubectl/pkg/drain filters.go, identical in v0.36.0 and v0.37.1).
// skipDeletedFilter is left out: it acts only with --skip-wait-for-delete-timeout,
// which draincheck does not accept.
var drainFilters = []drainFilter{daemonSetFilter, mirrorPodFilter, localStorageFilter, unreplicatedFilter}

// judgePod runs the filters as kubectl does: the first filter that does not
// return drainDelete decides, so a pod has one reason only; otherwise the last
// filter's answer stands (a warning from an earlier filter is dropped, as in
// kubectl).
func judgePod(pod corev1.Pod, o DrainOptions, owners *inventory.OwnerResolver) drainVerdict {
	var v drainVerdict
	for _, f := range drainFilters {
		v = f(pod, o, owners)
		if v.action != drainDelete {
			return v
		}
	}
	return v
}

// daemonSetFilter: kubectl looks the pod's DaemonSet up. A pod whose DaemonSet
// is gone is refused with a "not found" error, whatever --ignore-daemonsets
// says; with --force kubectl deletes it with a warning and goes on to the
// other filters. Otherwise the pod is refused, or skipped with
// --ignore-daemonsets. owners knows which DaemonSets exist; when the snapshot
// never collected them (or owners is nil) every DaemonSet counts as present.
func daemonSetFilter(pod corev1.Pod, o DrainOptions, owners *inventory.OwnerResolver) drainVerdict {
	ref := metav1.GetControllerOf(&pod)
	if ref == nil || ref.Kind != "DaemonSet" || ref.APIVersion != "apps/v1" || isFinished(pod) {
		return drainVerdict{}
	}
	if owners.DaemonSetGone(pod) {
		if o.Force {
			return drainVerdict{drainDelete, reasonDaemonSetGone}
		}
		return drainVerdict{drainRefuse, reasonDaemonSetGone}
	}
	if !o.IgnoreDaemonSets {
		return drainVerdict{drainRefuse, reasonDaemonSet}
	}
	return drainVerdict{drainSkip, reasonDaemonSet}
}

func mirrorPodFilter(pod corev1.Pod, _ DrainOptions, _ *inventory.OwnerResolver) drainVerdict {
	if isMirrorPod(pod) {
		return drainVerdict{drainSkip, reasonMirror}
	}
	return drainVerdict{}
}

func localStorageFilter(pod corev1.Pod, o DrainOptions, _ *inventory.OwnerResolver) drainVerdict {
	if !hasEmptyDir(pod) || isFinished(pod) || o.DeleteEmptyDirData {
		return drainVerdict{}
	}
	return drainVerdict{drainRefuse, reasonLocalStorage}
}

func unreplicatedFilter(pod corev1.Pod, o DrainOptions, _ *inventory.OwnerResolver) drainVerdict {
	if isFinished(pod) || metav1.GetControllerOf(&pod) != nil {
		return drainVerdict{}
	}
	if o.Force {
		return drainVerdict{drainDelete, reasonUnmanaged}
	}
	return drainVerdict{drainRefuse, reasonUnmanaged}
}

func hasEmptyDir(pod corev1.Pod) bool {
	for _, v := range pod.Spec.Volumes {
		if v.EmptyDir != nil {
			return true
		}
	}
	return false
}
