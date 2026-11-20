package analyzer

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	gpuTaint     = findings.NodeTaint{Key: "dedicated", Effect: "NoSchedule"}
	ssdSelector  = findings.LabelMismatch{Source: "node_selector", Key: "disktype", Operator: "In", Values: []string{"ssd"}, NodeHasKey: true, NodeValue: "hdd"}
	zoneRequired = func(have string) findings.LabelMismatch {
		return findings.LabelMismatch{Source: "node_affinity", Key: "zone", Operator: "In", Values: []string{"c"}, NodeHasKey: have != "", NodeValue: have}
	}
)

func combinedFinding(name string, nodes ...findings.RuledOutNode) findings.Finding {
	return findings.Finding{
		RuleID:   "WORKLOAD_PLACEMENT_COMBINED",
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: "StatefulSet", Namespace: "shop", Name: name},
		Evidence: findings.WorkloadPlacementCombinedEvidence{
			OwnerKind: "StatefulSet", OwnerName: name,
			Pods:           []findings.StrandedPod{{Name: name + "-0", Node: "worker"}},
			RemainingNodes: len(nodes),
			RuledOutNodes:  nodes,
		},
	}
}

func TestWorkloadPlacementCombined(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			// split: plain fails by label, gpu by taint. zoned: labels rule out both
			// nodes while plain is untainted, so it is the label rule's. free and
			// tolerant land somewhere.
			name:    "labels rule out one node and taints the other",
			targets: []string{"worker"},
			want: []findings.Finding{
				combinedFinding("split",
					findings.RuledOutNode{Name: "gpu", Taints: []findings.NodeTaint{gpuTaint}, Mismatches: []findings.LabelMismatch{}},
					findings.RuledOutNode{Name: "plain", Taints: []findings.NodeTaint{}, Mismatches: []findings.LabelMismatch{ssdSelector}}),
			},
		},
		{
			// Only gpu is left. free is taints alone (the taint rule's) and split is
			// taints alone too (gpu has the label). zoned is ruled out by the taint
			// and by the label on the same node: both causes, each enough.
			name:    "taints and labels each rule out the only node left",
			targets: []string{"worker", "plain"},
			want: []findings.Finding{
				combinedFinding("zoned",
					findings.RuledOutNode{Name: "gpu", Taints: []findings.NodeTaint{gpuTaint}, Mismatches: []findings.LabelMismatch{zoneRequired("")}}),
			},
		},
		{
			name:    "only plain is left: nothing is ruled out by both",
			targets: []string{"worker", "gpu"},
		},
		{
			name:    "no node is left at all",
			targets: []string{"worker", "plain", "gpu"},
		},
		{
			name:    "nothing moves",
			targets: []string{"plain"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := snapshotFromFixture(t, "workload-placement-combined.yaml")
			got := WorkloadPlacementCombined{}.Evaluate(Input{Snapshot: s, Targets: tt.targets})
			if !reflect.DeepEqual(got, withText(tt.want, got)) {
				t.Fatalf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

func TestWorkloadPlacementCombinedText(t *testing.T) {
	got := WorkloadPlacementCombined{}.Evaluate(Input{Snapshot: snapshotFromFixture(t, "workload-placement-combined.yaml"), Targets: []string{"worker"}})
	if len(got) != 1 {
		t.Fatalf("want 1 finding, got %d: %+v", len(got), got)
	}
	for _, want := range []string{"StatefulSet shop/split", "1 pod", "2 nodes", "gpu (taint dedicated:NoSchedule)", "plain (nodeSelector disktype=ssd, node has disktype=hdd)", "Fixing one cause"} {
		if !strings.Contains(got[0].Description, want) {
			t.Errorf("description %q lacks %q", got[0].Description, want)
		}
	}
	if why := catalogWhy(t, got[0]); !strings.Contains(why, "stay Pending") {
		t.Errorf("why %q lacks the consequence", why)
	}
	rem := catalogRemediation(t, got[0])
	for _, want := range []string{"kubectl get nodes --show-labels", "kubectl describe node", "toleration", "label"} {
		if !strings.Contains(rem, want) {
			t.Errorf("remediation %q lacks %q", rem, want)
		}
	}
}

func TestWorkloadPlacementCombinedNilSnapshotAndID(t *testing.T) {
	if got := (WorkloadPlacementCombined{}).Evaluate(Input{Targets: []string{"worker"}}); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
	if got := (WorkloadPlacementCombined{}).ID(); got != "WORKLOAD_PLACEMENT_COMBINED" {
		t.Errorf("ID() = %q", got)
	}
}

// The result must not depend on the order of the targets, nodes or pods, and the JSON has no null.
func TestWorkloadPlacementCombinedIgnoresOrderAndHasNoNull(t *testing.T) {
	s := snapshotFromFixture(t, "workload-placement-combined.yaml")
	want := WorkloadPlacementCombined{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker"}})
	if len(want) == 0 {
		t.Fatal("the fixture must produce a finding")
	}
	nodes, pods := s.Nodes(), s.Pods()
	slices.Reverse(nodes)
	slices.Reverse(pods)
	reversed := inventory.NewSnapshot(nodes, pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())
	got := WorkloadPlacementCombined{}.Evaluate(Input{Snapshot: reversed, Targets: []string{"worker"}})
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order changed the result\n got: %+v\nwant: %+v", got, want)
	}
	b, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "null") {
		t.Errorf("the JSON has a null: %s", b)
	}
}

// For every choice of targets, a workload that no node left can take is reported
// by one placement rule and never by two. When the taint rule defers to
// NO_REMAINING_NODES because every workload is stranded, a workload can have no
// placement finding; the other placement rules must not then report it twice.
func TestPlacementRulesReportEachStrandedWorkloadOnce(t *testing.T) {
	s := snapshotFromFixture(t, "workload-placement-combined.yaml")
	var names []string
	for _, n := range s.Nodes() {
		names = append(names, n.Name)
	}
	rules := []Rule{WorkloadUntoleratedTaints{}, WorkloadUnmatchedLabels{}, WorkloadPlacementCombined{}}
	reported := map[string]bool{}
	for mask := 1; mask < 1<<len(names); mask++ {
		var targets []string
		for i, n := range names {
			if mask&(1<<i) != 0 {
				targets = append(targets, n)
			}
		}
		in := Input{Snapshot: s, Targets: targets}
		seen := map[string]string{}
		for _, r := range rules {
			for _, f := range r.Evaluate(in) {
				key := f.Resource.String()
				if prev, dup := seen[key]; dup {
					t.Errorf("targets %v: %s is reported by %s and %s", targets, key, prev, r.ID())
				}
				seen[key] = r.ID()
				reported[r.ID()] = true
			}
		}
		// A stranded workload has a finding unless every workload is stranded and
		// the taint rule leaves the run to NO_REMAINING_NODES.
		moving := podsToMoveList(s, targets, DrainOptions{})
		left := remainingNodes(s, targets)
		groups := groupByWorkload(s, moving)
		var stranded []*workloadGroup
		for _, g := range groups {
			if !slices.ContainsFunc(left, func(n corev1.Node) bool { return placementOn(g.pods, n).Room }) {
				stranded = append(stranded, g)
			}
		}
		for _, g := range stranded {
			key := findings.ResourceRef{Kind: g.owner.Kind, Namespace: g.owner.Namespace, Name: g.owner.Name}.String()
			if _, ok := seen[key]; !ok && len(left) > 0 && len(stranded) != len(groups) {
				t.Errorf("targets %v: stranded workload %s is reported by no placement rule", targets, key)
			}
		}
	}
	for _, r := range rules {
		if !reported[r.ID()] {
			t.Errorf("no choice of targets made %s report: the fixture does not cover it", r.ID())
		}
	}
}

// edgeNode is a Ready, uncordoned node with labels and taints.
func edgeNode(name string, labels map[string]string, taints ...corev1.Taint) corev1.Node {
	n := taintedNode(name, taints...)
	n.Labels = labels
	return n
}

// reportedBy runs the three placement rules for the workload "app" of a cluster
// of a worker (the target), the given nodes and the pods, with a bystander
// workload that tolerates everything so that a run is never wholly stranded and
// the taint rule speaks. It returns the IDs of the rules that report "app".
func reportedBy(t *testing.T, nodes []corev1.Node, pods ...corev1.Pod) (ids []string, combined []findings.Finding) {
	t.Helper()
	bystander := ownedPod("bystander-0", "shop", "StatefulSet", "bystander", "worker", corev1.Toleration{Operator: corev1.TolerationOpExists})
	all := append([]corev1.Node{nodeWith("worker", false, corev1.ConditionTrue)}, nodes...)
	s := inventory.NewSnapshot(all, append([]corev1.Pod{bystander}, pods...), nil, nil, time.Time{})
	for _, r := range []Rule{WorkloadUntoleratedTaints{}, WorkloadUnmatchedLabels{}, WorkloadPlacementCombined{}} {
		for _, f := range r.Evaluate(Input{Snapshot: s, Targets: []string{"worker"}}) {
			if f.Resource.Name == "app" {
				ids = append(ids, r.ID())
				if r.ID() == WorkloadPlacementCombinedID {
					combined = append(combined, f)
				}
			}
		}
	}
	return ids, combined
}

func appPod(name string, mutate func(*corev1.Pod)) corev1.Pod {
	p := ownedPod(name, "shop", "StatefulSet", "app", "worker")
	if mutate != nil {
		mutate(&p)
	}
	return p
}

func requireAffinity(terms ...corev1.NodeSelectorTerm) *corev1.Affinity {
	return &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
		RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{NodeSelectorTerms: terms}}}
}

func exprTerm(key string, op corev1.NodeSelectorOperator, values ...string) corev1.NodeSelectorTerm {
	return corev1.NodeSelectorTerm{MatchExpressions: []corev1.NodeSelectorRequirement{{Key: key, Operator: op, Values: values}}}
}

// Edge cases of which of the three placement rules speaks. Exactly the rules in
// want report the workload "app".
func TestWorkloadPlacementCombinedEdgeCases(t *testing.T) {
	noSchedule := taint("dedicated", "fast", corev1.TaintEffectNoSchedule)
	noExecute := taint("dedicated", "fast", corev1.TaintEffectNoExecute)
	preferNo := taint("dedicated", "fast", corev1.TaintEffectPreferNoSchedule)
	ssd := map[string]string{"disktype": "ssd"}
	hdd := map[string]string{"disktype": "hdd"}
	selectSSD := func(p *corev1.Pod) { p.Spec.NodeSelector = ssd }

	tests := []struct {
		name  string
		nodes []corev1.Node
		pods  []corev1.Pod
		want  []string
	}{
		{"a NoExecute taint counts like NoSchedule", []corev1.Node{edgeNode("a", hdd), edgeNode("b", ssd, noExecute)},
			[]corev1.Pod{appPod("app-0", selectSSD)}, []string{WorkloadPlacementCombinedID}},
		{"a PreferNoSchedule taint rules nothing out: labels alone", []corev1.Node{edgeNode("a", hdd, preferNo), edgeNode("b", hdd)},
			[]corev1.Pod{appPod("app-0", selectSSD)}, []string{WorkloadUnmatchedLabelsID}},
		{"a PreferNoSchedule taint on the node that fits is room", []corev1.Node{edgeNode("a", hdd), edgeNode("b", ssd, preferNo)},
			[]corev1.Pod{appPod("app-0", selectSSD)}, nil},
		{"a constraint draincheck cannot evaluate is open: taints alone",
			[]corev1.Node{edgeNode("a", hdd, noSchedule), edgeNode("b", ssd, noSchedule)},
			[]corev1.Pod{appPod("app-0", func(p *corev1.Pod) {
				p.Spec.Affinity = requireAffinity(exprTerm("disktype", "Bogus", "ssd"))
			})}, []string{WorkloadUntoleratedTaintsID}},
		{"a preferred affinity rules nothing out: taints alone",
			[]corev1.Node{edgeNode("a", hdd, noSchedule)},
			[]corev1.Pod{appPod("app-0", func(p *corev1.Pod) {
				p.Spec.Affinity = &corev1.Affinity{NodeAffinity: &corev1.NodeAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{{Weight: 1, Preference: exprTerm("disktype", corev1.NodeSelectorOpIn, "ssd")}}}}
			})}, []string{WorkloadUntoleratedTaintsID}},
		{"a cordoned node that would fit is not room and is not listed",
			[]corev1.Node{edgeNode("a", hdd), edgeNode("b", ssd, noSchedule), func() corev1.Node {
				n := edgeNode("c", ssd)
				n.Spec.Unschedulable = true
				return n
			}()},
			[]corev1.Pod{appPod("app-0", selectSSD)}, []string{WorkloadPlacementCombinedID}},
		{"a NotReady node that would fit is not room and is not listed",
			[]corev1.Node{edgeNode("a", hdd), edgeNode("b", ssd, noSchedule), func() corev1.Node {
				n := edgeNode("c", ssd)
				n.Status.Conditions = []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionFalse}}
				return n
			}()},
			[]corev1.Pod{appPod("app-0", selectSSD)}, []string{WorkloadPlacementCombinedID}},
		{"one pod that fits keeps the workload out of every rule",
			[]corev1.Node{edgeNode("a", hdd), edgeNode("b", ssd, noSchedule)},
			[]corev1.Pod{appPod("app-0", selectSSD), appPod("app-1", nil)}, nil},
		{"a node that has the label and no taint is room",
			[]corev1.Node{edgeNode("a", hdd), edgeNode("b", ssd, noSchedule), edgeNode("c", ssd)},
			[]corev1.Pod{appPod("app-0", selectSSD)}, nil},
		{"a toleration for the taint turns the tainted node into room",
			[]corev1.Node{edgeNode("a", hdd), edgeNode("b", ssd, noSchedule)},
			[]corev1.Pod{appPod("app-0", func(p *corev1.Pod) {
				p.Spec.NodeSelector = ssd
				p.Spec.Tolerations = []corev1.Toleration{{Key: "dedicated", Operator: corev1.TolerationOpExists}}
			})}, nil},
		{"taints alone on the only node left, with another workload that fits: the taint rule",
			[]corev1.Node{edgeNode("a", hdd, noSchedule)},
			[]corev1.Pod{appPod("app-0", nil)}, []string{WorkloadUntoleratedTaintsID}},
		{"a pod that is already terminating is not moved: nothing to report",
			[]corev1.Node{edgeNode("a", hdd, noSchedule)},
			[]corev1.Pod{appPod("app-0", func(p *corev1.Pod) {
				now := metav1.Now()
				p.DeletionTimestamp = &now
				p.Spec.NodeSelector = ssd
			})}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := reportedBy(t, tt.nodes, tt.pods...)
			if !slices.Equal(got, tt.want) {
				t.Errorf("reported by %v, want %v", got, tt.want)
			}
		})
	}
}

// A node ruled out by a taint and a label at once lists both, taints first; a
// node left out of the report (cordoned, NotReady) is not listed; a toleration
// for one of two taints leaves only the other in the evidence.
func TestWorkloadPlacementCombinedEvidenceEdgeCases(t *testing.T) {
	a := taint("a", "", corev1.TaintEffectNoSchedule)
	b := taint("b", "", corev1.TaintEffectNoExecute)
	cordoned := edgeNode("cordoned", map[string]string{"disktype": "ssd"})
	cordoned.Spec.Unschedulable = true

	_, got := reportedBy(t,
		[]corev1.Node{edgeNode("both", map[string]string{"disktype": "hdd"}, a, b), edgeNode("plain", map[string]string{"disktype": "hdd"}), edgeNode("taint-only", map[string]string{"disktype": "ssd"}, a, b), cordoned},
		appPod("app-0", func(p *corev1.Pod) {
			p.Spec.NodeSelector = map[string]string{"disktype": "ssd"}
			p.Spec.Tolerations = []corev1.Toleration{{Key: "a", Operator: corev1.TolerationOpExists}}
		}))
	if len(got) != 1 {
		t.Fatalf("want 1 combined finding, got %+v", got)
	}
	ev := got[0].Evidence.(findings.WorkloadPlacementCombinedEvidence)
	if ev.RemainingNodes != 3 || len(ev.RuledOutNodes) != 3 {
		t.Fatalf("the cordoned node must not count or be listed: %+v", ev)
	}
	both, plain, taintOnly := ev.RuledOutNodes[0], ev.RuledOutNodes[1], ev.RuledOutNodes[2]
	wantTaints := []findings.NodeTaint{{Key: "b", Effect: "NoExecute"}}
	if both.Name != "both" || !reflect.DeepEqual(both.Taints, wantTaints) || len(both.Mismatches) != 1 {
		t.Errorf("node both: want taint b and one mismatch, got %+v", both)
	}
	if plain.Name != "plain" || plain.Taints == nil || len(plain.Taints) != 0 || len(plain.Mismatches) != 1 {
		t.Errorf("node plain: want an empty, non-nil taint list and one mismatch, got %+v", plain)
	}
	if taintOnly.Name != "taint-only" || !reflect.DeepEqual(taintOnly.Taints, wantTaints) || taintOnly.Mismatches == nil || len(taintOnly.Mismatches) != 0 {
		t.Errorf("node taint-only: want taint b and an empty, non-nil mismatch list, got %+v", taintOnly)
	}
}

// With several terms of a required affinity (ORed) the node lists the first
// unmet requirement of each, next to its taint.
func TestWorkloadPlacementCombinedAffinityTerms(t *testing.T) {
	_, got := reportedBy(t,
		[]corev1.Node{edgeNode("a", map[string]string{"disktype": "hdd", "zone": "a"}), edgeNode("b", map[string]string{"disktype": "ssd"}, taint("dedicated", "", corev1.TaintEffectNoSchedule))},
		appPod("app-0", func(p *corev1.Pod) {
			p.Spec.Affinity = requireAffinity(exprTerm("zone", corev1.NodeSelectorOpIn, "c"), exprTerm("disktype", corev1.NodeSelectorOpIn, "ssd"))
		}))
	// Node b has disktype=ssd, so the second term holds there and only the taint rules it out.
	if len(got) != 1 {
		t.Fatalf("want 1 combined finding, got %+v", got)
	}
	ev := got[0].Evidence.(findings.WorkloadPlacementCombinedEvidence)
	if n := len(ev.RuledOutNodes[0].Mismatches); n != 2 {
		t.Errorf("node a: want the unmet requirement of each of the 2 terms, got %d: %+v", n, ev.RuledOutNodes[0])
	}
	if len(ev.RuledOutNodes[1].Taints) != 1 || len(ev.RuledOutNodes[1].Mismatches) != 0 {
		t.Errorf("node b: want the taint only, got %+v", ev.RuledOutNodes[1])
	}
}

// The finding names the Deployment, not its ReplicaSet.
func TestWorkloadPlacementCombinedResolvesTheOwner(t *testing.T) {
	web := ownedPod("web-abc-1", "shop", "ReplicaSet", "web-abc", "worker")
	web.Spec.NodeSelector = map[string]string{"disktype": "ssd"}
	s := inventory.NewSnapshot(
		[]corev1.Node{nodeWith("worker", false, corev1.ConditionTrue), edgeNode("a", map[string]string{"disktype": "hdd"}), edgeNode("b", map[string]string{"disktype": "ssd"}, taint("x", "", corev1.TaintEffectNoSchedule))},
		[]corev1.Pod{web}, []appsv1.ReplicaSet{replicaSet("web-abc", "shop", "web")}, nil, time.Time{})
	got := WorkloadPlacementCombined{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker"}})
	if len(got) != 1 || got[0].Resource != (findings.ResourceRef{Kind: "Deployment", Namespace: "shop", Name: "web"}) {
		t.Fatalf("want one finding for Deployment shop/web, got %+v", got)
	}
}
