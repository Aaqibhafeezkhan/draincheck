package analyzer

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The eviction handler of kube-apiserver deletes a pod without consulting any
// budget when it has finished, has not started (Pending) or is already being
// deleted (canIgnorePDB).
func TestBypassesBudget(t *testing.T) {
	now := metav1.Now()
	tests := []struct {
		name string
		pod  corev1.Pod
		want bool
	}{
		{"running", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}, false},
		{"no phase reported", corev1.Pod{}, false},
		{"succeeded", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded}}, true},
		{"failed", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed}}, true},
		{"pending", corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}}, true},
		{"terminating", corev1.Pod{ObjectMeta: metav1.ObjectMeta{DeletionTimestamp: &now}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bypassesBudget(tt.pod); got != tt.want {
				t.Errorf("bypassesBudget = %v, want %v", got, tt.want)
			}
		})
	}
}
