package analyzer

import (
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func nodeWith(name string, cordoned bool, ready ...corev1.ConditionStatus) corev1.Node {
	n := corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}, Spec: corev1.NodeSpec{Unschedulable: cordoned}}
	for _, s := range ready {
		n.Status.Conditions = append(n.Status.Conditions, corev1.NodeCondition{Type: corev1.NodeReady, Status: s})
	}
	return n
}

func TestUnschedulable(t *testing.T) {
	tests := []struct {
		name string
		node corev1.Node
		want findings.UnschedulableNode
		ok   bool
	}{
		{"ready and schedulable", nodeWith("n", false, corev1.ConditionTrue), findings.UnschedulableNode{}, false},
		{"cordoned", nodeWith("n", true, corev1.ConditionTrue), findings.UnschedulableNode{Name: "n", Cordoned: true, ReadyStatus: "True"}, true},
		{"not ready", nodeWith("n", false, corev1.ConditionFalse), findings.UnschedulableNode{Name: "n", ReadyStatus: "False"}, true},
		{"ready unknown", nodeWith("n", false, corev1.ConditionUnknown), findings.UnschedulableNode{Name: "n", ReadyStatus: "Unknown"}, true},
		{"cordoned and not ready", nodeWith("n", true, corev1.ConditionFalse), findings.UnschedulableNode{Name: "n", Cordoned: true, ReadyStatus: "False"}, true},
		{"no Ready condition is not flagged", nodeWith("n", false), findings.UnschedulableNode{}, false},
		{"cordoned with no Ready condition", nodeWith("n", true), findings.UnschedulableNode{Name: "n", Cordoned: true}, true},
		{"other conditions are ignored", corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "n"},
			Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionTrue},
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
			}},
		}, findings.UnschedulableNode{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := unschedulable(tt.node)
			if ok != tt.ok || got != tt.want {
				t.Errorf("unschedulable() = %+v, %v; want %+v, %v", got, ok, tt.want, tt.ok)
			}
		})
	}
}
