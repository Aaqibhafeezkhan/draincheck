package analyzer

import (
	"testing"
	"time"

	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func selectionPod(name, node string, labels map[string]string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: name, Labels: labels},
		Spec:       corev1.PodSpec{NodeName: node},
	}
}

// The count is for the pods on the target nodes only, so the debug line says how
// many pods the selector kept in the drain and how many it left out.
func TestCountSelectedPods(t *testing.T) {
	pods := []corev1.Pod{
		selectionPod("a", "worker-01", map[string]string{"app": "web"}),
		selectionPod("b", "worker-01", map[string]string{"app": "db"}),
		selectionPod("c", "worker-01", nil),
		selectionPod("d", "worker-02", map[string]string{"app": "web"}),
		selectionPod("e", "worker-03", map[string]string{"app": "web"}), // not a target
	}
	s := inventory.NewSnapshot(nil, pods, nil, nil, time.Time{})
	targets := []string{"worker-01", "worker-02"}

	tests := []struct {
		name         string
		drain        DrainOptions
		wantSelected int
		wantLeftOut  int
	}{
		{"no selector selects every pod on the targets", DrainOptions{}, 4, 0},
		{"selector keeps the matching pods", DrainOptions{PodSelector: mustSelector(t, "app=web")}, 2, 2},
		{"selector that matches nothing", DrainOptions{PodSelector: mustSelector(t, "app=none")}, 0, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sel, left := CountSelectedPods(Input{Snapshot: s, Targets: targets, Drain: tt.drain})
			if sel != tt.wantSelected || left != tt.wantLeftOut {
				t.Errorf("selected, left out = %d, %d; want %d, %d", sel, left, tt.wantSelected, tt.wantLeftOut)
			}
		})
	}
}
