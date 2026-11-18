package analyzer

import (
	"reflect"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	want := nodePlacement{LabelRoom: true, Reasons: taintReasons(
		findings.NodeTaint{Key: "a", Effect: "NoSchedule"}, findings.NodeTaint{Key: "b", Effect: "NoExecute"})}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("no room: got %+v, want %+v", got, want)
	}
	if got := placementOn([]corev1.Pod{plain, tolAll}, node); !got.Room || !got.TaintRoom || !got.LabelRoom || got.Reasons != nil {
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
	want := nodePlacement{LabelRoom: true, Reasons: taintReasons(
		findings.NodeTaint{Key: "team", Effect: "NoSchedule"}, findings.NodeTaint{Key: "first", Effect: "NoSchedule"})}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// With no pod there is nothing to land: no room, and no reason to blame.
func TestPlacementOnNoPods(t *testing.T) {
	node := corev1.Node{Spec: corev1.NodeSpec{Taints: []corev1.Taint{taint("a", "", corev1.TaintEffectNoSchedule)}}}
	got := placementOn(nil, node)
	if got.Room || got.TaintRoom || got.LabelRoom || got.Reasons != nil {
		t.Errorf("got %+v, want no room of any kind and no reasons", got)
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
	// A reason of another kind is not a taint.
	other := nodePlacement{Reasons: []placementReason{{Kind: "other"}}}
	if got := other.taints(); got != nil {
		t.Errorf("a reason that is not a taint must not appear as one, got %+v", got)
	}
}

// Labels rule a node out on their own: taints leave room, labels do not.
func TestPlacementOnLabelsAlone(t *testing.T) {
	node := labelledNode("n1", map[string]string{"disktype": "hdd"})
	want := findings.LabelMismatch{Source: "node_selector", Key: "disktype", Operator: "In", Values: []string{"ssd"}, NodeHasKey: true, NodeValue: "hdd"}
	got := placementOn([]corev1.Pod{podWithSelector(map[string]string{"disktype": "ssd"})}, node)
	if got.Room || !got.TaintRoom || got.LabelRoom {
		t.Errorf("got %+v, want taint room only", got)
	}
	if !reflect.DeepEqual(got.labels(), []findings.LabelMismatch{want}) || got.taints() != nil {
		t.Errorf("labels = %+v, taints = %+v", got.labels(), got.taints())
	}
}

// One pod ruled out by a taint and another by a label: no room, though each
// cause leaves room to a pod. Room is for a pod, not for the two together.
func TestPlacementOnTaintForOnePodLabelForAnother(t *testing.T) {
	node := corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "n1", Labels: map[string]string{"disktype": "hdd"}},
		Spec: corev1.NodeSpec{Taints: []corev1.Taint{taint("a", "", corev1.TaintEffectNoSchedule)}}}
	byLabel := corev1.Pod{Spec: corev1.PodSpec{NodeSelector: map[string]string{"disktype": "ssd"},
		Tolerations: []corev1.Toleration{{Key: "a", Operator: corev1.TolerationOpExists}}}}
	byTaint := corev1.Pod{}
	got := placementOn([]corev1.Pod{byLabel, byTaint}, node)
	if got.Room || !got.TaintRoom || !got.LabelRoom {
		t.Errorf("got %+v, want no Room, TaintRoom and LabelRoom both true", got)
	}
	if len(got.taints()) != 1 || len(got.labels()) != 1 {
		t.Errorf("want one taint and one label reason, got %+v", got.Reasons)
	}
}

// Taints come first, then labels; the same requirement from two pods is one
// reason; the order of the pods does not change the labels.
func TestPlacementOnReasonOrderAndDistinct(t *testing.T) {
	node := corev1.Node{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{}},
		Spec: corev1.NodeSpec{Taints: []corev1.Taint{taint("a", "", corev1.TaintEffectNoSchedule)}}}
	zone := corev1.Pod{Spec: corev1.PodSpec{NodeSelector: map[string]string{"zone": "a"}}}
	disk := corev1.Pod{Spec: corev1.PodSpec{NodeSelector: map[string]string{"disktype": "ssd"}}}
	first := placementOn([]corev1.Pod{zone, disk, zone}, node)
	second := placementOn([]corev1.Pod{disk, zone}, node)
	if !reflect.DeepEqual(first, second) {
		t.Errorf("pod order changed the result\n%+v\n%+v", first, second)
	}
	if len(first.Reasons) != 3 || first.Reasons[0].Kind != placementReasonTaint ||
		first.Reasons[1].Label.Key != "disktype" || first.Reasons[2].Label.Key != "zone" {
		t.Errorf("want the taint, then disktype, then zone; got %+v", first.Reasons)
	}
}

// A requirement that cannot be evaluated is room, so it is never a reason.
func TestPlacementOnUnsupportedConstraintIsRoom(t *testing.T) {
	odd := podRequiring(corev1.NodeSelectorTerm{MatchFields: []corev1.NodeSelectorRequirement{expr("spec.unknown", corev1.NodeSelectorOpIn, "x")}})
	got := placementOn([]corev1.Pod{odd}, labelledNode("n1", nil))
	if !got.Room || got.Reasons != nil {
		t.Errorf("got %+v, want room", got)
	}
}
