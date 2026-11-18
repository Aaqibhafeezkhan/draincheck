package main

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/Veinar/draincheck/internal/inventory"
)

// labelledCluster has a target worker and one Ready, untainted node left with
// the label disktype=hdd; db-0 selects the given disktype.
func labelledCluster(selector string) *fake.Clientset {
	ready := []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}
	db := statefulSetPod("db-0", "db")
	db.Spec.NodeSelector = map[string]string{"disktype": selector}
	return fake.NewClientset(
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}, Status: corev1.NodeStatus{Conditions: ready}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-2", Labels: map[string]string{"disktype": "hdd"}}, Status: corev1.NodeStatus{Conditions: ready}},
		db,
	)
}

// A node selector no node left satisfies is reported on its own, with the rule's
// debug lines, and the whole-run rule stays silent because the node takes pods.
func TestExecuteWorkloadSelectorNoNodeMatchesIsReported(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnRisk}
	var out, logs bytes.Buffer
	err := execute(context.Background(), opts, debugLog(&logs), inventory.NewClientsetCollector(labelledCluster("ssd"), nil), &out)

	if exitCodeOf(err) != exitFindings {
		t.Errorf("exit code = %d (err %v), want %d", exitCodeOf(err), err, exitFindings)
	}
	ids := ruleIDs(t, out.Bytes())
	if !slices.Contains(ids, "WORKLOAD_UNMATCHED_NODE_LABELS") || slices.Contains(ids, "NO_REMAINING_NODES") || slices.Contains(ids, "WORKLOAD_UNTOLERATED_TAINTS") {
		t.Errorf("rules = %v, want only WORKLOAD_UNMATCHED_NODE_LABELS of the placement rules", ids)
	}
	for _, line := range []string{`msg="rule start" rule=WORKLOAD_UNMATCHED_NODE_LABELS`, `msg="rule done" rule=WORKLOAD_UNMATCHED_NODE_LABELS findings=1`} {
		if !strings.Contains(logs.String(), line) {
			t.Errorf("debug output lacks %q:\n%s", line, logs.String())
		}
	}
}

// A selector the node left satisfies gives no finding, and --debug does not change stdout.
func TestExecuteWorkloadSelectorMatchesIsSilent(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnRisk}
	var plain, debugged, logs bytes.Buffer
	c := atFixedTime{inventory.NewClientsetCollector(labelledCluster("hdd"), nil)}
	if err := execute(context.Background(), opts, nil, c, &plain); err != nil && exitCodeOf(err) != exitFindings {
		t.Fatal(err)
	}
	if err := execute(context.Background(), opts, debugLog(&logs), c, &debugged); err != nil && exitCodeOf(err) != exitFindings {
		t.Fatal(err)
	}
	if plain.String() != debugged.String() {
		t.Errorf("--debug changed stdout\n plain: %s\n debug: %s", plain.String(), debugged.String())
	}
	if ids := ruleIDs(t, plain.Bytes()); slices.Contains(ids, "WORKLOAD_UNMATCHED_NODE_LABELS") {
		t.Errorf("rules = %v, want no WORKLOAD_UNMATCHED_NODE_LABELS", ids)
	}
	if !strings.Contains(logs.String(), `msg="rule done" rule=WORKLOAD_UNMATCHED_NODE_LABELS findings=0`) {
		t.Errorf("debug output lacks the rule's done line:\n%s", logs.String())
	}
}
