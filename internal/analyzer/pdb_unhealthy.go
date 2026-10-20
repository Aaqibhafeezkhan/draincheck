package analyzer

import (
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
)

// unhealthyPolicy returns the effective unhealthyPodEvictionPolicy of pdb: an
// unset field means IfHealthyBudget. The second result is false for a value
// draincheck does not know, which is not guessed.
func unhealthyPolicy(pdb policyv1.PodDisruptionBudget) (policyv1.UnhealthyPodEvictionPolicyType, bool) {
	policy := pdb.Spec.UnhealthyPodEvictionPolicy
	if policy == nil {
		return policyv1.IfHealthyBudget, true
	}
	switch *policy {
	case policyv1.IfHealthyBudget, policyv1.AlwaysAllow:
		return *policy, true
	}
	return *policy, false
}

// podReady reports whether the Ready condition of pod is True. A pod with no
// Ready condition is not ready, as in the eviction handler.
func podReady(pod corev1.Pod) bool {
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady {
			return cond.Status == corev1.ConditionTrue
		}
	}
	return false
}

// evictedPastBudget reports whether the eviction API deletes pod without using
// up pdb, although it selects the pod: the pod is not Ready and the policy
// lets it go. AlwaysAllow always does; IfHealthyBudget does while the guarded
// application is not disrupted (currentHealthy >= desiredHealthy and
// desiredHealthy > 0). A Ready pod always goes through the budget, and a value
// of the policy draincheck does not know lets nothing past it.
//
// The answer rests on the numbers in the status of pdb, so a caller must not
// ask it for a budget whose status cannot be trusted.
func evictedPastBudget(pod corev1.Pod, pdb policyv1.PodDisruptionBudget) bool {
	if podReady(pod) {
		return false
	}
	policy, known := unhealthyPolicy(pdb)
	if !known {
		return false
	}
	if policy == policyv1.AlwaysAllow {
		return true
	}
	return pdb.Status.CurrentHealthy >= pdb.Status.DesiredHealthy && pdb.Status.DesiredHealthy > 0
}
