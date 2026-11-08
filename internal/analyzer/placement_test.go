package analyzer

import (
	"reflect"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
)

func taintReasons(taints ...findings.NodeTaint) []placementReason {
	out := make([]placementReason, len(taints))
	for i, t := range taints {
		out[i] = placementReason{Kind: placementReasonTaint, Taint: t}
	}
	return out
}

func TestPlacementOn(t *testing.T) {
	node := corev1.Node{Spec: corev1.NodeSpec{Taints: []corev1.Taint{
		taint("a", "", corev1.TaintEffectNoSchedule),
		taint("b", "", corev1.TaintEffectNoExecute),
		taint("c", "", corev1.TaintEffectPreferNoSchedule),
	}}}
	plain := corev1.Pod{}
	tolA := corev1.Pod{Spec: corev1.PodSpec{Tolerations: []corev1.Toleration{{Key: "a", Operator: corev1.TolerationOpExists}}}}
	tolAll := corev1.Pod{Spec: corev1.PodSpec{Tolerations: []corev1.Toleration{{Operator: corev1.TolerationOpExists}}}}

	got := placementOn([]corev1.Pod{plain, tolA}, node)
	want := nodePlacement{Reasons: taintReasons(
		findings.NodeTaint{Key: "a", Effect: "NoSchedule"}, findings.NodeTaint{Key: "b", Effect: "NoExecute"})}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("no room: got %+v, want %+v", got, want)
	}
	if got := placementOn([]corev1.Pod{plain, tolAll}, node); !got.Room || got.Reasons != nil {
		t.Errorf("one pod that tolerates everything is room with no reasons, got %+v", got)
	}
	if got := placementOn([]corev1.Pod{plain}, corev1.Node{}); !got.Room {
		t.Error("an untainted node is room")
	}
}

// Two taints with the same key and effect but different values are one reason,
// in the order the node lists them.
func TestPlacementOnSameKeyAndEffect(t *testing.T) {
	node := corev1.Node{Spec: corev1.NodeSpec{Taints: []corev1.Taint{
		taint("team", "a", corev1.TaintEffectNoSchedule),
		taint("first", "", corev1.TaintEffectNoSchedule),
		taint("team", "b", corev1.TaintEffectNoSchedule),
	}}}
	got := placementOn([]corev1.Pod{{}}, node)
	want := nodePlacement{Reasons: taintReasons(
		findings.NodeTaint{Key: "team", Effect: "NoSchedule"}, findings.NodeTaint{Key: "first", Effect: "NoSchedule"})}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// With no pod there is nothing to land: no room, and no reason to blame.
func TestPlacementOnNoPods(t *testing.T) {
	node := corev1.Node{Spec: corev1.NodeSpec{Taints: []corev1.Taint{taint("a", "", corev1.TaintEffectNoSchedule)}}}
	got := placementOn(nil, node)
	if got.Room || got.Reasons != nil {
		t.Errorf("got %+v, want no room and no reasons", got)
	}
}

func TestNodePlacementTaints(t *testing.T) {
	p := nodePlacement{Reasons: taintReasons(findings.NodeTaint{Key: "a", Effect: "NoSchedule"})}
	want := []findings.NodeTaint{{Key: "a", Effect: "NoSchedule"}}
	if got := p.taints(); !reflect.DeepEqual(got, want) {
		t.Errorf("taints = %+v, want %+v", got, want)
	}
	if got := (nodePlacement{Room: true}).taints(); got != nil {
		t.Errorf("no reasons, no taints; got %+v", got)
	}
	// A reason of another kind is not a taint (labels arrive with #8).
	other := nodePlacement{Reasons: []placementReason{{Kind: "other"}}}
	if got := other.taints(); got != nil {
		t.Errorf("a reason that is not a taint must not appear as one, got %+v", got)
	}
}
