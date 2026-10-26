package analyzer

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func mustSelector(t *testing.T, s string) labels.Selector {
	t.Helper()
	sel, err := labels.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return sel
}

// --pod-selector is a label selector, as in kubectl: an empty one selects every
// pod, and so does no selector at all.
func TestDrainOptionsSelects(t *testing.T) {
	pod := corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web", "tier": "front"}}}
	unlabeled := corev1.Pod{}
	tests := []struct {
		name     string
		selector labels.Selector
		pod      corev1.Pod
		want     bool
	}{
		{"no selector selects every pod", nil, pod, true},
		{"no selector selects an unlabeled pod", nil, unlabeled, true},
		{"empty selector selects every pod", mustSelector(t, ""), pod, true},
		{"equality match", mustSelector(t, "app=web"), pod, true},
		{"equality mismatch", mustSelector(t, "app=db"), pod, false},
		{"set-based match", mustSelector(t, "tier in (front,back)"), pod, true},
		{"both requirements must match", mustSelector(t, "app=web,tier=back"), pod, false},
		{"negation matches an unlabeled pod", mustSelector(t, "app!=web"), unlabeled, true},
		{"existence matches a labeled pod", mustSelector(t, "app"), pod, true},
		{"existence does not match an unlabeled pod", mustSelector(t, "app"), unlabeled, false},
		{"absence matches an unlabeled pod", mustSelector(t, "!app"), unlabeled, true},
		{"absence does not match a labeled pod", mustSelector(t, "!app"), pod, false},
		{"notin matches a pod without the key", mustSelector(t, "app notin (web)"), unlabeled, true},
		{"spaces around the selector are ignored", mustSelector(t, "  app = web  "), pod, true},
		{"double equals is equality", mustSelector(t, "app==web"), pod, true},
		{"label values are compared exactly", mustSelector(t, "app=We"), pod, false},
		{"equality does not match an unlabeled pod", mustSelector(t, "app=web"), unlabeled, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := (DrainOptions{PodSelector: tt.selector}).selects(tt.pod); got != tt.want {
				t.Errorf("selects = %v, want %v", got, tt.want)
			}
		})
	}
}
