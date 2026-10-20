package analyzer

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
)

func podWithReady(status corev1.ConditionStatus) corev1.Pod {
	return corev1.Pod{Status: corev1.PodStatus{
		Phase:      corev1.PodRunning,
		Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: status}},
	}}
}

func TestPodReady(t *testing.T) {
	if !podReady(podWithReady(corev1.ConditionTrue)) {
		t.Error("Ready=True must be ready")
	}
	if podReady(podWithReady(corev1.ConditionFalse)) {
		t.Error("Ready=False must not be ready")
	}
	if podReady(corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}) {
		t.Error("a pod with no Ready condition must not be ready, as in the eviction handler")
	}
}

func TestUnhealthyPolicy(t *testing.T) {
	policy := func(p policyv1.UnhealthyPodEvictionPolicyType) policyv1.PodDisruptionBudget {
		return policyv1.PodDisruptionBudget{Spec: policyv1.PodDisruptionBudgetSpec{UnhealthyPodEvictionPolicy: &p}}
	}
	tests := []struct {
		name   string
		pdb    policyv1.PodDisruptionBudget
		want   policyv1.UnhealthyPodEvictionPolicyType
		wantOK bool
	}{
		{"unset is IfHealthyBudget", policyv1.PodDisruptionBudget{}, policyv1.IfHealthyBudget, true},
		{"IfHealthyBudget", policy(policyv1.IfHealthyBudget), policyv1.IfHealthyBudget, true},
		{"AlwaysAllow", policy(policyv1.AlwaysAllow), policyv1.AlwaysAllow, true},
		{"an unknown value is not guessed", policy("SometimesAllow"), "SometimesAllow", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := unhealthyPolicy(tt.pdb)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("unhealthyPolicy = %q, %v; want %q, %v", got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

// A pod that is not Ready is evicted past its budget with AlwaysAllow, and with
// IfHealthyBudget only while currentHealthy >= desiredHealthy and
// desiredHealthy > 0. A Ready pod always goes through the budget.
func TestEvictedPastBudget(t *testing.T) {
	pdb := func(policy *policyv1.UnhealthyPodEvictionPolicyType, healthy, desired int32) policyv1.PodDisruptionBudget {
		return policyv1.PodDisruptionBudget{
			Spec:   policyv1.PodDisruptionBudgetSpec{UnhealthyPodEvictionPolicy: policy},
			Status: policyv1.PodDisruptionBudgetStatus{CurrentHealthy: healthy, DesiredHealthy: desired},
		}
	}
	always, ifHealthy := policyv1.AlwaysAllow, policyv1.IfHealthyBudget
	ready, notReady := podWithReady(corev1.ConditionTrue), podWithReady(corev1.ConditionFalse)

	tests := []struct {
		name string
		pod  corev1.Pod
		pdb  policyv1.PodDisruptionBudget
		want bool
	}{
		{"ready pod, healthy budget", ready, pdb(nil, 2, 2), false},
		{"ready pod, AlwaysAllow", ready, pdb(&always, 1, 2), false},
		{"unready pod, AlwaysAllow, unhealthy budget", notReady, pdb(&always, 1, 2), true},
		{"unready pod, default policy, healthy budget", notReady, pdb(nil, 2, 2), true},
		{"unready pod, IfHealthyBudget, healthy budget", notReady, pdb(&ifHealthy, 3, 2), true},
		{"unready pod, IfHealthyBudget, unhealthy budget", notReady, pdb(&ifHealthy, 1, 2), false},
		{"unready pod, default policy, desired 0", notReady, pdb(nil, 0, 0), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := evictedPastBudget(tt.pod, tt.pdb); got != tt.want {
				t.Errorf("evictedPastBudget = %v, want %v", got, tt.want)
			}
		})
	}
}
