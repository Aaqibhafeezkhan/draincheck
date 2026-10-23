package analyzer

import (
	"github.com/Veinar/draincheck/internal/findings"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
)

func taint(key, value string, effect corev1.TaintEffect) corev1.Taint {
	return corev1.Taint{Key: key, Value: value, Effect: effect}
}

func TestUntoleratedTaints(t *testing.T) {
	cp := taint("node-role.kubernetes.io/control-plane", "", corev1.TaintEffectNoSchedule)
	gpu := taint("dedicated", "gpu", corev1.TaintEffectNoExecute)
	soft := taint("prefer", "", corev1.TaintEffectPreferNoSchedule)

	tests := []struct {
		name        string
		taints      []corev1.Taint
		tolerations []corev1.Toleration
		want        []corev1.Taint
	}{
		{"no taints", nil, nil, nil},
		{"NoSchedule without a toleration", []corev1.Taint{cp}, nil, []corev1.Taint{cp}},
		{"NoExecute without a toleration", []corev1.Taint{gpu}, nil, []corev1.Taint{gpu}},
		{"PreferNoSchedule never removes room", []corev1.Taint{soft}, nil, nil},
		{"Exists on the key tolerates every effect", []corev1.Taint{cp},
			[]corev1.Toleration{{Key: cp.Key, Operator: corev1.TolerationOpExists}}, nil},
		{"Equal needs the same value", []corev1.Taint{gpu},
			[]corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "cpu"}}, []corev1.Taint{gpu}},
		{"Equal with the value tolerates", []corev1.Taint{gpu},
			[]corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpEqual, Value: "gpu", Effect: corev1.TaintEffectNoExecute}}, nil},
		{"an effect that differs does not tolerate", []corev1.Taint{gpu},
			[]corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoSchedule}}, []corev1.Taint{gpu}},
		{"empty key with Exists tolerates everything", []corev1.Taint{cp, gpu},
			[]corev1.Toleration{{Operator: corev1.TolerationOpExists}}, nil},
		{"tolerationSeconds is ignored", []corev1.Taint{gpu},
			[]corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpExists, TolerationSeconds: new(int64)}}, nil},
		{"only the untolerated taints are returned, in node order", []corev1.Taint{cp, gpu},
			[]corev1.Toleration{{Key: cp.Key, Operator: corev1.TolerationOpExists}}, []corev1.Taint{gpu}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := corev1.Node{Spec: corev1.NodeSpec{Taints: tt.taints}}
			pod := corev1.Pod{Spec: corev1.PodSpec{Tolerations: tt.tolerations}}
			if got := untoleratedTaints(pod, node); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("untoleratedTaints = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestTaintsAgainst(t *testing.T) {
	node := corev1.Node{Spec: corev1.NodeSpec{Taints: []corev1.Taint{
		taint("a", "", corev1.TaintEffectNoSchedule),
		taint("b", "", corev1.TaintEffectNoExecute),
		taint("c", "", corev1.TaintEffectPreferNoSchedule),
	}}}
	plain := corev1.Pod{}
	tolA := corev1.Pod{Spec: corev1.PodSpec{Tolerations: []corev1.Toleration{{Key: "a", Operator: corev1.TolerationOpExists}}}}
	tolAll := corev1.Pod{Spec: corev1.PodSpec{Tolerations: []corev1.Toleration{{Operator: corev1.TolerationOpExists}}}}

	got, room := taintsAgainst([]corev1.Pod{plain, tolA}, node)
	want := []findings.NodeTaint{{Key: "a", Effect: "NoSchedule"}, {Key: "b", Effect: "NoExecute"}}
	if room || !reflect.DeepEqual(got, want) {
		t.Errorf("no room: got %+v room=%v, want %+v", got, room, want)
	}
	if _, room := taintsAgainst([]corev1.Pod{plain, tolAll}, node); !room {
		t.Error("one pod that tolerates everything is room")
	}
	if _, room := taintsAgainst([]corev1.Pod{plain}, corev1.Node{}); !room {
		t.Error("an untainted node is room")
	}
}

// taintedNode is a Ready, uncordoned node with the given taints.
func taintedNode(name string, taints ...corev1.Taint) corev1.Node {
	n := nodeWith(name, false, corev1.ConditionTrue)
	n.Spec.Taints = taints
	return n
}

// ownedPod is a running pod on node, controlled by owner of the given kind.
func ownedPod(name, namespace, kind, owner, node string, tolerations ...corev1.Toleration) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, OwnerReferences: []metav1.OwnerReference{
			{APIVersion: "apps/v1", Kind: kind, Name: owner, Controller: ptr.To(true)}}},
		Spec:   corev1.PodSpec{NodeName: node, Tolerations: tolerations},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}

// defaultTolerations are the two the API server adds to every pod.
var defaultTolerations = []corev1.Toleration{
	{Key: "node.kubernetes.io/not-ready", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute, TolerationSeconds: ptr.To(int64(300))},
	{Key: "node.kubernetes.io/unreachable", Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute, TolerationSeconds: ptr.To(int64(300))},
}

func TestUntoleratedTaintsMatchingEdges(t *testing.T) {
	cp := taint("node-role.kubernetes.io/control-plane", "", corev1.TaintEffectNoSchedule)
	gpu := taint("dedicated", "gpu", corev1.TaintEffectNoExecute)
	level := taint("level", "5", corev1.TaintEffectNoSchedule)
	notReady := taint("node.kubernetes.io/not-ready", "", corev1.TaintEffectNoExecute)

	tests := []struct {
		name        string
		taints      []corev1.Taint
		tolerations []corev1.Toleration
		want        []corev1.Taint
	}{
		{"an empty operator means Equal: same value tolerates", []corev1.Taint{gpu},
			[]corev1.Toleration{{Key: "dedicated", Value: "gpu"}}, nil},
		{"an empty operator means Equal: another value does not", []corev1.Taint{gpu},
			[]corev1.Toleration{{Key: "dedicated", Value: "cpu"}}, []corev1.Taint{gpu}},
		{"an empty operator, a taint with no value and a toleration with none", []corev1.Taint{cp},
			[]corev1.Toleration{{Key: cp.Key}}, nil},
		{"Exists with no key and NoExecute tolerates NoExecute taints only", []corev1.Taint{cp, gpu},
			[]corev1.Toleration{{Operator: corev1.TolerationOpExists, Effect: corev1.TaintEffectNoExecute}}, []corev1.Taint{cp}},
		{"a taint with an effect the scheduler does not know is ignored", []corev1.Taint{taint("odd", "", corev1.TaintEffect("Sometimes"))}, nil, nil},
		{"Gt tolerates a taint value above its own", []corev1.Taint{level},
			[]corev1.Toleration{{Key: "level", Operator: corev1.TolerationOpGt, Value: "3"}}, nil},
		{"Gt does not tolerate an equal value", []corev1.Taint{level},
			[]corev1.Toleration{{Key: "level", Operator: corev1.TolerationOpGt, Value: "5"}}, []corev1.Taint{level}},
		{"Lt tolerates a taint value below its own", []corev1.Taint{level},
			[]corev1.Toleration{{Key: "level", Operator: corev1.TolerationOpLt, Value: "7"}}, nil},
		{"Lt with a value that is not a number does not tolerate", []corev1.Taint{level},
			[]corev1.Toleration{{Key: "level", Operator: corev1.TolerationOpLt, Value: "x"}}, []corev1.Taint{level}},
		{"the tolerations every pod gets do not tolerate a control plane", []corev1.Taint{cp}, defaultTolerations, []corev1.Taint{cp}},
		{"the tolerations every pod gets do tolerate not-ready", []corev1.Taint{notReady}, defaultTolerations, nil},
		{"an unknown operator tolerates nothing", []corev1.Taint{cp},
			[]corev1.Toleration{{Key: cp.Key, Operator: corev1.TolerationOperator("Maybe")}}, []corev1.Taint{cp}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := corev1.Node{Spec: corev1.NodeSpec{Taints: tt.taints}}
			pod := corev1.Pod{Spec: corev1.PodSpec{Tolerations: tt.tolerations}}
			if got := untoleratedTaints(pod, node); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("untoleratedTaints = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// Two taints with the same key and effect but different values are one entry
// in the evidence, in the order the node lists them.
func TestTaintsAgainstSameKeyAndEffect(t *testing.T) {
	node := corev1.Node{Spec: corev1.NodeSpec{Taints: []corev1.Taint{
		taint("team", "a", corev1.TaintEffectNoSchedule),
		taint("first", "", corev1.TaintEffectNoSchedule),
		taint("team", "b", corev1.TaintEffectNoSchedule),
	}}}
	got, room := taintsAgainst([]corev1.Pod{{}}, node)
	want := []findings.NodeTaint{{Key: "team", Effect: "NoSchedule"}, {Key: "first", Effect: "NoSchedule"}}
	if room || !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v room=%v, want %+v", got, room, want)
	}
}

// With no pod there is nothing to land: no room, and no taint to blame.
func TestTaintsAgainstNoPods(t *testing.T) {
	node := corev1.Node{Spec: corev1.NodeSpec{Taints: []corev1.Taint{taint("a", "", corev1.TaintEffectNoSchedule)}}}
	got, room := taintsAgainst(nil, node)
	if room || got != nil {
		t.Errorf("got %+v room=%v, want nil and false", got, room)
	}
}
