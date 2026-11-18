package analyzer

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
)

func unmatchedFinding(name string, mismatched ...findings.MismatchedNode) findings.Finding {
	return findings.Finding{
		RuleID:   "WORKLOAD_UNMATCHED_NODE_LABELS",
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: "StatefulSet", Namespace: "shop", Name: name},
		Evidence: findings.WorkloadUnmatchedLabelsEvidence{
			OwnerKind: "StatefulSet", OwnerName: name,
			Pods:            []findings.StrandedPod{{Name: name + "-0", Node: "worker"}},
			RemainingNodes:  len(mismatched),
			MismatchedNodes: mismatched,
		},
	}
}

func selectorMismatch(have string) []findings.LabelMismatch {
	return []findings.LabelMismatch{{Source: "node_selector", Key: "disktype", Operator: "In", Values: []string{"ssd"}, NodeHasKey: true, NodeValue: have}}
}

func zoneMismatch(zone string) []findings.LabelMismatch {
	return []findings.LabelMismatch{{Source: "node_affinity", Key: "zone", Operator: "In", Values: []string{"c"}, NodeHasKey: zone != "", NodeValue: zone}}
}

func TestWorkloadUnmatchedLabels(t *testing.T) {
	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			// cache selects disktype=ssd, which only the target "fast" has; search
			// requires zone c, which no node has. free, soft, odd and agent are fine.
			name:    "a selector and a required affinity that no node left satisfies",
			targets: []string{"worker", "fast"},
			want: []findings.Finding{
				unmatchedFinding("cache",
					findings.MismatchedNode{Name: "node-a", Mismatches: selectorMismatch("hdd")},
					findings.MismatchedNode{Name: "node-b", Mismatches: selectorMismatch("hdd")}),
				unmatchedFinding("search",
					findings.MismatchedNode{Name: "node-a", Mismatches: zoneMismatch("a")},
					findings.MismatchedNode{Name: "node-b", Mismatches: zoneMismatch("b")}),
			},
		},
		{
			// "fast" is left now and has the label, but a taint rules cache out
			// there: labels and taints together, which neither rule states alone.
			name:    "labels leave only a node that a taint rules out: neither cause alone",
			targets: []string{"worker"},
			want: []findings.Finding{
				unmatchedFinding("search",
					findings.MismatchedNode{Name: "fast", Mismatches: []findings.LabelMismatch{{Source: "node_affinity", Key: "zone", Operator: "In", Values: []string{"c"}}}},
					findings.MismatchedNode{Name: "node-a", Mismatches: zoneMismatch("a")},
					findings.MismatchedNode{Name: "node-b", Mismatches: zoneMismatch("b")}),
			},
		},
		{
			// Taints rule out the only node left, so labels are not the cause on
			// their own: NO_REMAINING_NODES states the run.
			name:    "taints rule out every node left: not a label finding",
			targets: []string{"worker", "node-a", "node-b"},
		},
		{
			name:    "no node is left at all",
			targets: []string{"worker", "node-a", "node-b", "fast"},
		},
		{
			name:    "nothing moves",
			targets: []string{"node-a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := snapshotFromFixture(t, "workload-unmatched-labels.yaml")
			got := WorkloadUnmatchedLabels{}.Evaluate(Input{Snapshot: s, Targets: tt.targets})
			if !reflect.DeepEqual(got, withText(tt.want, got)) {
				t.Fatalf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

func TestWorkloadUnmatchedLabelsText(t *testing.T) {
	got := WorkloadUnmatchedLabels{}.Evaluate(Input{Snapshot: snapshotFromFixture(t, "workload-unmatched-labels.yaml"), Targets: []string{"worker", "fast"}})
	if len(got) != 2 {
		t.Fatalf("want 2 findings, got %d", len(got))
	}
	for _, want := range []string{"StatefulSet shop/cache", "1 pod", "2 nodes", "node-a", "nodeSelector disktype=ssd", "has disktype=hdd"} {
		if !strings.Contains(got[0].Description, want) {
			t.Errorf("description %q lacks %q", got[0].Description, want)
		}
	}
	for _, want := range []string{"required node affinity zone In [c]", "has zone=b"} {
		if !strings.Contains(got[1].Description, want) {
			t.Errorf("description %q lacks %q", got[1].Description, want)
		}
	}
	if why := catalogWhy(t, got[0]); !strings.Contains(why, "stay Pending") {
		t.Errorf("why %q lacks the consequence", why)
	}
	if !strings.Contains(catalogRemediation(t, got[0]), "kubectl get nodes --show-labels") {
		t.Errorf("remediation %q lacks the label check", catalogRemediation(t, got[0]))
	}
}

func TestWorkloadUnmatchedLabelsNilSnapshotAndID(t *testing.T) {
	if got := (WorkloadUnmatchedLabels{}).Evaluate(Input{Targets: []string{"worker"}}); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
	if got := (WorkloadUnmatchedLabels{}).ID(); got != "WORKLOAD_UNMATCHED_NODE_LABELS" {
		t.Errorf("ID() = %q", got)
	}
}

// The result must not depend on the order of the targets, nodes or pods, and the JSON has no null.
func TestWorkloadUnmatchedLabelsIgnoresOrderAndHasNoNull(t *testing.T) {
	s := snapshotFromFixture(t, "workload-unmatched-labels.yaml")
	want := WorkloadUnmatchedLabels{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker", "fast"}})

	nodes, pods := s.Nodes(), s.Pods()
	slices.Reverse(nodes)
	slices.Reverse(pods)
	reversed := inventory.NewSnapshot(nodes, pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())
	got := WorkloadUnmatchedLabels{}.Evaluate(Input{Snapshot: reversed, Targets: []string{"fast", "worker"}})
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

// A workload is judged as a whole: one pod that fits a node keeps it out of the report.
func TestWorkloadUnmatchedLabelsAnyPodThatFitsIsRoom(t *testing.T) {
	s := snapshotFromFixture(t, "workload-unmatched-labels.yaml")
	var pods []corev1.Pod
	for _, p := range s.Pods() {
		pods = append(pods, p)
		if p.Name == "cache-0" {
			twin := *p.DeepCopy()
			twin.Name = "cache-1"
			twin.Spec.NodeSelector = nil // an older revision without the selector
			pods = append(pods, twin)
		}
	}
	s = inventory.NewSnapshot(s.Nodes(), pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())
	for _, f := range (WorkloadUnmatchedLabels{}).Evaluate(Input{Snapshot: s, Targets: []string{"worker", "fast"}}) {
		if f.Resource.Name == "cache" {
			t.Errorf("cache has a pod that can land, it must not be reported: %+v", f)
		}
	}
}
