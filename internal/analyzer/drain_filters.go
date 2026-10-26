package analyzer

import (
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
type drainFilter func(pod corev1.Pod, o DrainOptions) drainVerdict

// drainFilters are kubectl's pod filters in the order it applies them
// (k8s.io/kubectl/pkg/drain filters.go, identical in v0.36.0 and v0.37.1).
// skipDeletedFilter is left out: it acts only with --skip-wait-for-delete-timeout,
// which draincheck does not accept.
var drainFilters = []drainFilter{daemonSetFilter, mirrorPodFilter, localStorageFilter, unreplicatedFilter}

// judgePod runs the filters as kubectl does: the first filter that does not
// return drainDelete decides, so a pod has one reason only; otherwise the last
// filter's answer stands (a warning from an earlier filter is dropped, as in
// kubectl).
func judgePod(pod corev1.Pod, o DrainOptions) drainVerdict {
	var v drainVerdict
	for _, f := range drainFilters {
		v = f(pod, o)
		if v.action != drainDelete {
			return v
		}
	}
	return v
}

// daemonSetFilter: kubectl also looks the DaemonSet up and refuses a pod whose
// DaemonSet is gone (deleting it with --force). draincheck does not read
// DaemonSets, so such a pod is treated as DaemonSet-managed; the garbage
// collector removes these pods within seconds.
func daemonSetFilter(pod corev1.Pod, o DrainOptions) drainVerdict {
	ref := metav1.GetControllerOf(&pod)
	if ref == nil || ref.Kind != "DaemonSet" || ref.APIVersion != "apps/v1" || isFinished(pod) {
		return drainVerdict{}
	}
	if !o.IgnoreDaemonSets {
		return drainVerdict{drainRefuse, reasonDaemonSet}
	}
	return drainVerdict{drainSkip, reasonDaemonSet}
}

func mirrorPodFilter(pod corev1.Pod, _ DrainOptions) drainVerdict {
	if isMirrorPod(pod) {
		return drainVerdict{drainSkip, reasonMirror}
	}
	return drainVerdict{}
}

func localStorageFilter(pod corev1.Pod, o DrainOptions) drainVerdict {
	if !hasEmptyDir(pod) || isFinished(pod) || o.DeleteEmptyDirData {
		return drainVerdict{}
	}
	return drainVerdict{drainRefuse, reasonLocalStorage}
}

func unreplicatedFilter(pod corev1.Pod, o DrainOptions) drainVerdict {
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
