package analyzer

import (
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
)

// mirrorPodAnnotation is set by the kubelet on the API object that mirrors a
// static pod.
const mirrorPodAnnotation = "kubernetes.io/config.mirror"

// podClass is what a pod is, whatever state it is in. POD_OWNERSHIP reports by
// class: kubectl drain applies its DaemonSet, static and unreplicated checks to
// a pod that is already terminating as well.
type podClass int

const (
	// classFinished is a pod in a terminal phase (Succeeded, Failed).
	classFinished podClass = iota
	// classStatic is a static (mirror) pod.
	classStatic
	// classDaemonSet is a pod controlled by a DaemonSet.
	classDaemonSet
	// classUnmanaged is a pod with no controller.
	classUnmanaged
	// classManaged is a pod controlled by anything else.
	classManaged
)

// drainFate is what a drain does with a pod on a target node. It is the answer
// every rule asks for; no rule looks at the phase, the deletion timestamp or
// the mirror annotation on its own.
type drainFate int

const (
	// fateNotSelected: --pod-selector does not match the pod, so kubectl drain
	// does not list it and leaves it as it is.
	fateNotSelected drainFate = iota
	// fateFinished: the pod has run to completion. Drain removes it without
	// further checks and nothing needs a new home.
	fateFinished
	// fateLeaving: the pod is already being deleted. Its controller has already
	// created the replacement, so the drain does not take anything down.
	fateLeaving
	// fateLeftAlone: a DaemonSet or static pod. kubectl drain skips it (with
	// --ignore-daemonsets for the first), so it is never evicted.
	fateLeftAlone
	// fateEvictedUnmanaged: drain evicts the pod and nothing recreates it.
	fateEvictedUnmanaged
	// fateEvictedRescheduled: drain evicts the pod and its controller recreates
	// it elsewhere.
	fateEvictedRescheduled
)

// String gives the reason, for a debug line.
func (f drainFate) String() string {
	switch f {
	case fateNotSelected:
		return "not selected by --pod-selector"
	case fateFinished:
		return "finished (Succeeded or Failed)"
	case fateLeaving:
		return "already terminating"
	case fateLeftAlone:
		return "left alone by kubectl drain (DaemonSet or static pod)"
	case fateEvictedUnmanaged:
		return "evicted, not recreated (no controller)"
	case fateEvictedRescheduled:
		return "evicted, recreated by its controller"
	default:
		return "unknown"
	}
}

// podDrain is the classification of one pod.
type podDrain struct {
	// selected is false for a pod that --pod-selector does not match.
	selected    bool
	class       podClass
	terminating bool
	pending     bool
}

// classifyPod decides what a drain with the given flags does with pod. A pod
// that --pod-selector does not match is not part of the drain. A finished pod is finished
// whatever else is true of it; a static pod is static even when it carries a
// DaemonSet or Node owner reference.
func classifyPod(pod corev1.Pod, owners *inventory.OwnerResolver, drain DrainOptions) podDrain {
	d := podDrain{
		selected:    drain.selects(pod),
		terminating: pod.DeletionTimestamp != nil,
		pending:     pod.Status.Phase == corev1.PodPending,
	}
	owner := owners.Resolve(pod)
	switch {
	case isFinished(pod):
		d.class = classFinished
	case isMirrorPod(pod):
		d.class = classStatic
	case owner.Kind == "DaemonSet":
		d.class = classDaemonSet
	case owner.Unmanaged():
		d.class = classUnmanaged
	default:
		d.class = classManaged
	}
	return d
}

// isFinished reports whether the pod is in a terminal phase (Succeeded,
// Failed), which kubectl drain removes without further checks.
func isFinished(pod corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
}

// isMirrorPod reports whether the pod is the API object of a static pod.
func isMirrorPod(pod corev1.Pod) bool {
	return hasAnnotation(pod.Annotations, mirrorPodAnnotation)
}

// fate is what the drain does with the pod. Order matters: not selected, then
// finished, then already leaving, then left alone, then evicted.
func (d podDrain) fate() drainFate {
	switch {
	case !d.selected:
		return fateNotSelected
	case d.class == classFinished:
		return fateFinished
	case d.terminating:
		return fateLeaving
	case d.class == classStatic || d.class == classDaemonSet:
		return fateLeftAlone
	case d.class == classUnmanaged:
		return fateEvictedUnmanaged
	default:
		return fateEvictedRescheduled
	}
}

// bypassesBudget reports whether the eviction API deletes the pod without
// consulting any PodDisruptionBudget. kube-apiserver does so for a pod that has
// finished (Succeeded, Failed), has not started (Pending) or is already being
// deleted (canIgnorePDB in the eviction handler). Such a pod is neither refused
// by a budget nor counted against one, and it does not take part in the "more
// than one budget" refusal either.
func (d podDrain) bypassesBudget() bool {
	return d.class == classFinished || d.pending || d.terminating
}

// consultsBudget reports whether a budget is consulted for the pod: drain
// evicts it and the eviction API does not let it past.
func (d podDrain) consultsBudget() bool {
	return d.selected && !d.bypassesBudget() && d.class != classStatic && d.class != classDaemonSet
}
