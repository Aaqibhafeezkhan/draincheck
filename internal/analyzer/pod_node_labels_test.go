package analyzer

import (
	"reflect"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func labelledNode(name string, labels map[string]string) corev1.Node {
	return corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

func podWithSelector(sel map[string]string) corev1.Pod {
	return corev1.Pod{Spec: corev1.PodSpec{NodeSelector: sel}}
}

// podRequiring is a pod whose required node affinity has the given terms.
func podRequiring(terms ...corev1.NodeSelectorTerm) corev1.Pod {
	return corev1.Pod{Spec: corev1.PodSpec{Affinity: &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: terms}}}}}
}

func expr(key string, op corev1.NodeSelectorOperator, values ...string) corev1.NodeSelectorRequirement {
	return corev1.NodeSelectorRequirement{Key: key, Operator: op, Values: values}
}

func term(exprs ...corev1.NodeSelectorRequirement) corev1.NodeSelectorTerm {
	return corev1.NodeSelectorTerm{MatchExpressions: exprs}
}

func TestUnmatchedNodeLabelsSelector(t *testing.T) {
	node := labelledNode("n1", map[string]string{"disktype": "hdd", "zone": "a"})
	tests := []struct {
		name string
		pod  corev1.Pod
		want []findings.LabelMismatch
	}{
		{"no selector", corev1.Pod{}, nil},
		{"selector matches", podWithSelector(map[string]string{"disktype": "hdd"}), nil},
		{"value differs", podWithSelector(map[string]string{"disktype": "ssd"}), []findings.LabelMismatch{
			{Source: "node_selector", Key: "disktype", Operator: "In", Values: []string{"ssd"}, NodeHasKey: true, NodeValue: "hdd"}}},
		{"key missing", podWithSelector(map[string]string{"gpu": "yes"}), []findings.LabelMismatch{
			{Source: "node_selector", Key: "gpu", Operator: "In", Values: []string{"yes"}}}},
		{"every unmet key, sorted by key", podWithSelector(map[string]string{"zone": "b", "disktype": "ssd"}), []findings.LabelMismatch{
			{Source: "node_selector", Key: "disktype", Operator: "In", Values: []string{"ssd"}, NodeHasKey: true, NodeValue: "hdd"},
			{Source: "node_selector", Key: "zone", Operator: "In", Values: []string{"b"}, NodeHasKey: true, NodeValue: "a"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unmatchedNodeLabels(tt.pod, node); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestUnmatchedNodeLabelsAffinity(t *testing.T) {
	node := labelledNode("n1", map[string]string{"zone": "a", "cores": "8"})
	mm := func(key, op string, has bool, value string, wanted ...string) findings.LabelMismatch {
		return findings.LabelMismatch{Source: "node_affinity", Key: key, Operator: op, Values: append([]string{}, wanted...), NodeHasKey: has, NodeValue: value}
	}
	tests := []struct {
		name string
		pod  corev1.Pod
		want []findings.LabelMismatch
	}{
		{"In matches", podRequiring(term(expr("zone", corev1.NodeSelectorOpIn, "a", "b"))), nil},
		{"In does not match", podRequiring(term(expr("zone", corev1.NodeSelectorOpIn, "c"))), []findings.LabelMismatch{mm("zone", "In", true, "a", "c")}},
		{"In on a missing key", podRequiring(term(expr("rack", corev1.NodeSelectorOpIn, "r1"))), []findings.LabelMismatch{mm("rack", "In", false, "", "r1")}},
		{"NotIn matches when the value is not listed", podRequiring(term(expr("zone", corev1.NodeSelectorOpNotIn, "b"))), nil},
		{"NotIn matches when the key is missing", podRequiring(term(expr("rack", corev1.NodeSelectorOpNotIn, "r1"))), nil},
		{"NotIn fails when the value is listed", podRequiring(term(expr("zone", corev1.NodeSelectorOpNotIn, "a"))), []findings.LabelMismatch{mm("zone", "NotIn", true, "a", "a")}},
		{"Exists matches", podRequiring(term(expr("zone", corev1.NodeSelectorOpExists))), nil},
		{"Exists fails on a missing key", podRequiring(term(expr("rack", corev1.NodeSelectorOpExists))), []findings.LabelMismatch{mm("rack", "Exists", false, "")}},
		{"DoesNotExist matches a missing key", podRequiring(term(expr("rack", corev1.NodeSelectorOpDoesNotExist))), nil},
		{"DoesNotExist fails on a present key", podRequiring(term(expr("zone", corev1.NodeSelectorOpDoesNotExist))), []findings.LabelMismatch{mm("zone", "DoesNotExist", true, "a")}},
		{"Gt matches", podRequiring(term(expr("cores", corev1.NodeSelectorOpGt, "4"))), nil},
		{"Gt fails", podRequiring(term(expr("cores", corev1.NodeSelectorOpGt, "8"))), []findings.LabelMismatch{mm("cores", "Gt", true, "8", "8")}},
		{"Lt matches", podRequiring(term(expr("cores", corev1.NodeSelectorOpLt, "16"))), nil},
		{"Gt on a label that is not a number fails", podRequiring(term(expr("zone", corev1.NodeSelectorOpGt, "4"))), []findings.LabelMismatch{mm("zone", "Gt", true, "a", "4")}},
		{"expressions of a term are ANDed: the first unmet one is reported", podRequiring(term(
			expr("zone", corev1.NodeSelectorOpIn, "a"), expr("rack", corev1.NodeSelectorOpExists), expr("cores", corev1.NodeSelectorOpGt, "99"))),
			[]findings.LabelMismatch{mm("rack", "Exists", false, "")}},
		{"terms are ORed: one that matches is room", podRequiring(
			term(expr("zone", corev1.NodeSelectorOpIn, "c")), term(expr("zone", corev1.NodeSelectorOpIn, "a"))), nil},
		{"no term matches: one mismatch per term", podRequiring(
			term(expr("zone", corev1.NodeSelectorOpIn, "c")), term(expr("rack", corev1.NodeSelectorOpExists))),
			[]findings.LabelMismatch{mm("zone", "In", true, "a", "c"), mm("rack", "Exists", false, "")}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unmatchedNodeLabels(tt.pod, node); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestUnmatchedNodeLabelsMatchFields(t *testing.T) {
	node := labelledNode("n1", nil)
	byName := func(op corev1.NodeSelectorOperator, names ...string) corev1.Pod {
		return podRequiring(corev1.NodeSelectorTerm{MatchFields: []corev1.NodeSelectorRequirement{expr("metadata.name", op, names...)}})
	}
	if got := unmatchedNodeLabels(byName(corev1.NodeSelectorOpIn, "n1"), node); got != nil {
		t.Errorf("metadata.name In [n1] matches n1, got %+v", got)
	}
	want := []findings.LabelMismatch{{Source: "node_affinity_field", Key: "metadata.name", Operator: "In", Values: []string{"n2"}, NodeHasKey: true, NodeValue: "n1"}}
	if got := unmatchedNodeLabels(byName(corev1.NodeSelectorOpIn, "n2"), node); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// What draincheck cannot evaluate counts as open: nothing is reported on a guess.
func TestUnmatchedNodeLabelsUnsupportedIsOpen(t *testing.T) {
	node := labelledNode("n1", map[string]string{"zone": "a"})
	tests := []struct {
		name string
		pod  corev1.Pod
	}{
		{"unknown operator", podRequiring(term(expr("zone", "Weird", "a")))},
		{"In with no values", podRequiring(term(expr("zone", corev1.NodeSelectorOpIn)))},
		{"Gt with two values", podRequiring(term(expr("zone", corev1.NodeSelectorOpGt, "1", "2")))},
		{"Gt with a value that is not a number", podRequiring(term(expr("zone", corev1.NodeSelectorOpGt, "many")))},
		{"matchFields on another field", podRequiring(corev1.NodeSelectorTerm{MatchFields: []corev1.NodeSelectorRequirement{expr("spec.unknown", corev1.NodeSelectorOpIn, "x")}})},
		{"a term with nothing in it", podRequiring(corev1.NodeSelectorTerm{})},
		{"no terms at all", podRequiring()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := unmatchedNodeLabels(tt.pod, node); got != nil {
				t.Errorf("want open (nil), got %+v", got)
			}
		})
	}
}

// An unmet expression decides a term even when another one in it cannot be evaluated.
func TestUnmatchedNodeLabelsUnmetBeatsUnsupported(t *testing.T) {
	node := labelledNode("n1", map[string]string{"zone": "a"})
	pod := podRequiring(term(expr("zone", "Weird", "a"), expr("rack", corev1.NodeSelectorOpExists)))
	want := []findings.LabelMismatch{{Source: "node_affinity", Key: "rack", Operator: "Exists", Values: []string{}}}
	if got := unmatchedNodeLabels(pod, node); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// Preferred terms and a pod with no affinity rule nothing out; a nil node label map is fine.
func TestUnmatchedNodeLabelsPreferredAndNilLabels(t *testing.T) {
	preferred := corev1.Pod{Spec: corev1.PodSpec{Affinity: &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
		PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{{Weight: 1, Preference: term(expr("zone", corev1.NodeSelectorOpIn, "c"))}}}}}}
	if got := unmatchedNodeLabels(preferred, labelledNode("n1", nil)); got != nil {
		t.Errorf("a preference rules out nothing, got %+v", got)
	}
	if got := unmatchedNodeLabels(corev1.Pod{Spec: corev1.PodSpec{Affinity: &corev1.Affinity{}}}, labelledNode("n1", nil)); got != nil {
		t.Errorf("an empty affinity rules out nothing, got %+v", got)
	}
	want := []findings.LabelMismatch{{Source: "node_selector", Key: "a", Operator: "In", Values: []string{"b"}}}
	if got := unmatchedNodeLabels(podWithSelector(map[string]string{"a": "b"}), labelledNode("n1", nil)); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}
