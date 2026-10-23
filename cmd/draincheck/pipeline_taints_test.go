package main

import (
	"bytes"
	"context"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/ptr"

	"github.com/Veinar/draincheck/internal/inventory"
)

// taintedControlPlaneCluster is a worker holding one StatefulSet pod that
// tolerates nothing, and a Ready control plane tainted NoSchedule: the commonest
// small cluster, where draining the worker leaves the pod nowhere to go.
func taintedControlPlaneCluster(extra ...*corev1.Pod) *fake.Clientset {
	ready := []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}
	objs := []runtime.Object{
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}, Status: corev1.NodeStatus{Conditions: ready}},
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "control-plane"},
			Spec:       corev1.NodeSpec{Taints: []corev1.Taint{{Key: "node-role.kubernetes.io/control-plane", Effect: corev1.TaintEffectNoSchedule}}},
			Status:     corev1.NodeStatus{Conditions: ready},
		},
		statefulSetPod("db-0", "db"),
	}
	for _, p := range extra {
		objs = append(objs, p)
	}
	return fake.NewClientset(objs...)
}

func statefulSetPod(name, owner string, tolerations ...corev1.Toleration) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: name, OwnerReferences: []metav1.OwnerReference{
			{APIVersion: "apps/v1", Kind: "StatefulSet", Name: owner, Controller: ptr.To(true)}}},
		Spec: corev1.PodSpec{NodeName: "worker-1", Tolerations: tolerations},
		Status: corev1.PodStatus{Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}},
	}
}

func TestExecuteTaintedControlPlaneLeavesNoRoom(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnRisk}
	var out bytes.Buffer
	err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(taintedControlPlaneCluster(), nil), &out)

	if exitCodeOf(err) != exitFindings {
		t.Errorf("exit code = %d (err %v), want %d", exitCodeOf(err), err, exitFindings)
	}
	if ids := ruleIDs(t, out.Bytes()); !slices.Contains(ids, "NO_REMAINING_NODES") {
		t.Errorf("rules = %v, want NO_REMAINING_NODES among them", ids)
	}
}

func TestExecuteTaintedControlPlaneDebugDoesNotChangeStdout(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnRisk}
	c := atFixedTime{inventory.NewClientsetCollector(taintedControlPlaneCluster(), nil)}
	var plain, debug, logs bytes.Buffer

	_ = execute(context.Background(), opts, nil, c, &plain)
	_ = execute(context.Background(), opts, debugLog(&logs), c, &debug)

	if !bytes.Equal(plain.Bytes(), debug.Bytes()) {
		t.Errorf("stdout differs with a debug logger\nwithout:\n%s\nwith:\n%s", plain.String(), debug.String())
	}
}
