package analyzer

import corev1 "k8s.io/api/core/v1"

// bypassesBudget reports whether the eviction API deletes pod without
// consulting any PodDisruptionBudget. kube-apiserver does so for a pod that has
// finished (Succeeded, Failed), has not started (Pending) or is already being
// deleted (canIgnorePDB in the eviction handler). Such a pod is neither refused
// by a budget nor counted against one, and it does not take part in the
// "more than one budget" refusal either.
func bypassesBudget(pod corev1.Pod) bool {
	return terminal(pod) || pod.Status.Phase == corev1.PodPending || pod.DeletionTimestamp != nil
}
