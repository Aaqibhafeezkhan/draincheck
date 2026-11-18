package analyzer

import (
	"reflect"
	"testing"
	"time"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
)

func TestGroupByWorkloadSortsAndSeparatesNamespaces(t *testing.T) {
	pods := []corev1.Pod{
		ownedPod("db-0", "team-b", "StatefulSet", "db", "worker"),
		ownedPod("web-0", "team-a", "StatefulSet", "web", "worker"),
		ownedPod("db-1", "team-b", "StatefulSet", "db", "worker"),
		ownedPod("db-0", "team-a", "StatefulSet", "db", "worker"),
	}
	groups := groupByWorkload(inventory.NewSnapshot(nil, pods, nil, nil, time.Time{}), pods)
	var got []string
	for _, g := range groups {
		got = append(got, g.owner.Namespace+"/"+g.owner.Name)
	}
	if want := []string{"team-a/db", "team-a/web", "team-b/db"}; !reflect.DeepEqual(got, want) {
		t.Errorf("groups = %v, want %v", got, want)
	}
	if len(groups[2].pods) != 2 {
		t.Errorf("team-b/db has %d pods, want 2", len(groups[2].pods))
	}
	if got := groupByWorkload(inventory.NewSnapshot(nil, nil, nil, nil, time.Time{}), nil); len(got) != 0 {
		t.Errorf("no pods, no groups; got %v", got)
	}
}

// roomOf asks each cause alone: taint room is not label room, and no node means no room.
func TestWorkloadGroupRoomOf(t *testing.T) {
	tainted := corev1.Node{Spec: corev1.NodeSpec{Taints: []corev1.Taint{taint("a", "", corev1.TaintEffectNoSchedule)}}}
	tainted.Name, tainted.Labels = "tainted", map[string]string{"disktype": "ssd"}
	plain := labelledNode("plain", map[string]string{"disktype": "hdd"})
	pod := podWithSelector(map[string]string{"disktype": "ssd"})
	g := &workloadGroup{pods: []corev1.Pod{pod}}

	if got := g.roomOf([]corev1.Node{tainted, plain}); got != (groupRoom{taint: true, label: true}) {
		t.Errorf("each cause leaves a node open (on different nodes): got %+v", got)
	}
	if got := g.roomOf([]corev1.Node{plain}); got != (groupRoom{taint: true}) {
		t.Errorf("labels rule out the only node: got %+v", got)
	}
	if got := g.roomOf([]corev1.Node{tainted}); got != (groupRoom{label: true}) {
		t.Errorf("a taint rules out the only node: got %+v", got)
	}
	if got := g.roomOf(nil); got != (groupRoom{}) {
		t.Errorf("no node, no room: got %+v", got)
	}
}

func TestWorkloadGroupStrandedPodsSorted(t *testing.T) {
	g := &workloadGroup{pods: []corev1.Pod{
		ownedPod("b", "shop", "StatefulSet", "x", "w2"), ownedPod("a", "shop", "StatefulSet", "x", "w2"), ownedPod("a", "shop", "StatefulSet", "x", "w1"),
	}}
	want := []findings.StrandedPod{{Name: "a", Node: "w1"}, {Name: "a", Node: "w2"}, {Name: "b", Node: "w2"}}
	if got := g.strandedPods(); !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
	if got := (&workloadGroup{}).strandedPods(); got == nil || len(got) != 0 {
		t.Errorf("an empty group gives an empty, non-nil list (JSON has no null); got %#v", got)
	}
}
