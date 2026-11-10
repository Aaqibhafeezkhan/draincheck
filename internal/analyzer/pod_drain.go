package analyzer

import (
	"time"

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
	// classOrphanedDaemonSet is a pod controlled by a DaemonSet that no longer
	// exists (kubectl drain looks it up). It is evicted, not skipped, and nothing
	// recreates it.
	classOrphanedDaemonSet
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
		return "evicted, not recreated (no controller, or its DaemonSet is gone)"
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
	// skippedByKubectl is a static pod, or a DaemonSet pod that has not
	// finished: kubectl drain skips it. A finished DaemonSet pod is not skipped:
	// the drain waits for it (observed with kubectl v1.36.1). Whether it skips a
	// finished static pod is not observed; none is reported.
	skippedByKubectl bool
	// evictionDisabled is --disable-eviction: kubectl deletes the pods instead
	// of evicting them, so no budget is consulted for any pod.
	evictionDisabled bool
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

		evictionDisabled: drain.DisableEviction,
	}
	owner := owners.Resolve(pod)
	d.skippedByKubectl = isMirrorPod(pod) || (owner.Kind == "DaemonSet" && !owners.DaemonSetGone(pod) && !isFinished(pod))
	switch {
	case isFinished(pod):
		d.class = classFinished
	case isMirrorPod(pod):
		d.class = classStatic
	case owners.DaemonSetGone(pod):
		d.class = classOrphanedDaemonSet
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
	case d.class == classUnmanaged || d.class == classOrphanedDaemonSet:
		return fateEvictedUnmanaged
	default:
		return fateEvictedRescheduled
	}
}

// waitsForDeletion reports whether kubectl drain waits for the pod to be gone
// after it is deleted. It does so for every pod it acts on, a finished pod
// included: a finished pod is deleted, not evicted, and a finalizer keeps it in
// the API (observed with kubectl v1.36.1: a Completed pod held by a finalizer,
// DaemonSet pod included, made the drain give up at --timeout). A running
// DaemonSet pod and a static pod are skipped, and a pod --pod-selector leaves
// out is not listed.
func (d podDrain) waitsForDeletion() bool {
	return d.selected && !d.skippedByKubectl
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
// evicts it and the eviction API does not let it past. With --disable-eviction
// drain deletes the pod and the eviction API is not called, so none is.
func (d podDrain) consultsBudget() bool {
	return !d.evictionDisabled && d.selected && !d.bypassesBudget() && d.class != classStatic && d.class != classDaemonSet
}

// terminatingDeadline returns the time by which a pod that is being deleted
// should be gone: metadata.deletionTimestamp, which the API server sets to the
// time deletion started plus the grace period. ok is false for a pod that is
// not being deleted.
func terminatingDeadline(pod corev1.Pod) (deadline time.Time, ok bool) {
	if pod.DeletionTimestamp == nil {
		return time.Time{}, false
	}
	return pod.DeletionTimestamp.Time, true
}

// terminationGrace returns the pod's own grace period, terminationGracePeriodSeconds,
// else the 30 s default. The recorded deletionGracePeriodSeconds is not used: once
// the container has exited the kubelet sets it to 0 (observed), which says nothing
// about the grace the pod was given.
func terminationGrace(pod corev1.Pod) int64 {
	if pod.Spec.TerminationGracePeriodSeconds != nil {
		return *pod.Spec.TerminationGracePeriodSeconds
	}
	return 30
}
