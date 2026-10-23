package main

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/Veinar/draincheck/internal/inventory"
)

// With a second pod on the worker that tolerates the control-plane taint, the
// run is not wholly stranded: the workload that tolerates nothing is reported
// on its own, and the whole-run rule stays silent.
func TestExecuteWorkloadWithoutTolerationIsReportedAlone(t *testing.T) {
	proxy := statefulSetPod("proxy-0", "proxy", corev1.Toleration{Key: "node-role.kubernetes.io/control-plane", Operator: corev1.TolerationOpExists})
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnRisk}
	var out, logs bytes.Buffer
	err := execute(context.Background(), opts, debugLog(&logs), inventory.NewClientsetCollector(taintedControlPlaneCluster(proxy), nil), &out)

	if exitCodeOf(err) != exitFindings {
		t.Errorf("exit code = %d (err %v), want %d", exitCodeOf(err), err, exitFindings)
	}
	ids := ruleIDs(t, out.Bytes())
	if !slices.Contains(ids, "WORKLOAD_UNTOLERATED_TAINTS") || slices.Contains(ids, "NO_REMAINING_NODES") {
		t.Errorf("rules = %v, want WORKLOAD_UNTOLERATED_TAINTS and not NO_REMAINING_NODES", ids)
	}
	for _, line := range []string{`msg="rule start" rule=WORKLOAD_UNTOLERATED_TAINTS`, `msg="rule done" rule=WORKLOAD_UNTOLERATED_TAINTS findings=1`} {
		if !strings.Contains(logs.String(), line) {
			t.Errorf("debug output lacks %q:\n%s", line, logs.String())
		}
	}
}
