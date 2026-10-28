package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"github.com/Veinar/draincheck/internal/inventory"
)

// unmanagedCluster has one bare pod on worker-1: a POD_OWNERSHIP finding, RISK
// with --force and BLOCKED without it.
func unmanagedCluster() *fake.Clientset {
	return fake.NewClientset(
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-2"}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "bare"},
			Spec:       corev1.PodSpec{NodeName: "worker-1"},
		},
	)
}

func TestExecute(t *testing.T) {
	tests := []struct {
		name     string
		opts     options
		wantCode int
		wantRule bool
	}{
		{"risk passes at blocked threshold", options{Nodes: []string{"worker-1"}, Force: true, Output: outputJSON, FailOn: failOnBlocked}, 0, true},
		{"risk fails at risk threshold", options{Nodes: []string{"worker-1"}, Force: true, Output: outputJSON, FailOn: failOnRisk}, exitFindings, true},
		{"the pod with no controller is BLOCKED without --force", options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked}, exitFindings, true},
		{"no pods on target, clean", options{Nodes: []string{"worker-2"}, Force: true, Output: outputJSON, FailOn: failOnAny}, 0, false},
		{"wildcard targets every node", options{AllNodes: true, Force: true, Output: outputJSON, FailOn: failOnRisk}, exitFindings, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := execute(context.Background(), tt.opts, nil, inventory.NewClientsetCollector(unmanagedCluster(), nil), &out)

			if tt.wantCode == 0 && err != nil {
				t.Fatalf("execute() error = %v, want nil", err)
			}
			if tt.wantCode != 0 && exitCodeOf(err) != tt.wantCode {
				t.Fatalf("exit code = %d (err %v), want %d", exitCodeOf(err), err, tt.wantCode)
			}

			var doc struct {
				SchemaVersion string `json:"schema_version"`
				Findings      []struct {
					RuleID string `json:"rule_id"`
				} `json:"findings"`
			}
			if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
				t.Fatalf("output is not JSON: %v\n%s", err, out.String())
			}
			if doc.SchemaVersion != "v1" {
				t.Errorf("schema_version = %q", doc.SchemaVersion)
			}
			if got := len(doc.Findings) > 0; got != tt.wantRule {
				t.Errorf("has findings = %v, want %v", got, tt.wantRule)
			}
		})
	}
}

func TestExecuteUnknownNode(t *testing.T) {
	var out bytes.Buffer
	opts := options{Nodes: []string{"ghost"}, Output: outputJSON, FailOn: failOnBlocked}
	err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(unmanagedCluster(), nil), &out)

	var nf *inventory.NodesNotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("error = %v, want *NodesNotFoundError", err)
	}
	if exitCodeOf(err) != exitFailure {
		t.Errorf("exit code = %d, want %d", exitCodeOf(err), exitFailure)
	}
	if out.Len() != 0 {
		t.Errorf("wrote output on error: %s", out.String())
	}
}

// fixedTime is the snapshot time used by tests that compare whole reports.
var fixedTime = time.Date(2026, 10, 14, 9, 30, 0, 0, time.UTC)

// atFixedTime re-stamps the snapshots of inner, so reports are reproducible.
type atFixedTime struct{ inner inventory.Collector }

func (c atFixedTime) Collect(ctx context.Context) (*inventory.Snapshot, error) {
	s, err := c.inner.Collect(ctx)
	if err != nil {
		return nil, err
	}
	return inventory.NewSnapshot(s.Nodes(), s.Pods(), s.ReplicaSets(), s.PodDisruptionBudgets(), fixedTime), nil
}

func TestExecuteReportsSnapshotTimestampAndDisclaimer(t *testing.T) {
	for _, format := range []string{outputText, outputJSON} {
		t.Run(format, func(t *testing.T) {
			opts := options{Nodes: []string{"worker-1"}, Output: format, FailOn: failOnBlocked}
			var out bytes.Buffer

			_ = execute(context.Background(), opts, nil, atFixedTime{inventory.NewClientsetCollector(unmanagedCluster(), nil)}, &out)

			for _, want := range []string{"2026-10-14T09:30:00Z", "does not guarantee execution success"} {
				if !strings.Contains(out.String(), want) {
					t.Errorf("report is missing %q:\n%s", want, out.String())
				}
			}
		})
	}
}

func TestExecuteTextWritesReadableReport(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputText, FailOn: failOnBlocked}
	var out bytes.Buffer
	err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(unmanagedCluster(), nil), &out)
	if err != nil && exitCodeOf(err) != exitFindings {
		t.Fatalf("error = %v, want nil or exit code %d", err, exitFindings)
	}
	if !strings.Contains(out.String(), "=== FINDINGS SUMMARY ===") {
		t.Errorf("output is not the text report:\n%s", out.String())
	}
	if json.Valid(out.Bytes()) {
		t.Errorf("--output text must not write JSON:\n%s", out.String())
	}
}

func TestExecuteVerboseSelectsFullTextLayout(t *testing.T) {
	for _, tc := range []struct {
		verbose bool
		want    string
	}{
		{false, "[POD_OWNERSHIP] 1 finding"},
		{true, "[POD_OWNERSHIP] Pod/default/bare"},
	} {
		opts := options{Nodes: []string{"worker-1"}, Output: outputText, FailOn: failOnBlocked, Force: true, Verbose: tc.verbose}
		var out bytes.Buffer
		_ = execute(context.Background(), opts, nil, inventory.NewClientsetCollector(unmanagedCluster(), nil), &out)
		if !strings.Contains(out.String(), tc.want) {
			t.Errorf("verbose=%v: missing %q:\n%s", tc.verbose, tc.want, out.String())
		}
	}
}

type failingCollector struct{}

func (failingCollector) Collect(context.Context) (*inventory.Snapshot, error) {
	return nil, errors.New("boom")
}

func TestExecuteCollectError(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked}
	err := execute(context.Background(), opts, nil, failingCollector{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("error = %v, want wrapped collector error", err)
	}
	if exitCodeOf(err) != exitFailure {
		t.Errorf("exit code = %d, want %d", exitCodeOf(err), exitFailure)
	}
}

// ruleIDs returns the rule IDs in a JSON report.
func ruleIDs(t *testing.T, raw []byte) []string {
	t.Helper()
	var doc struct {
		Findings []struct {
			RuleID string `json:"rule_id"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, raw)
	}
	ids := make([]string, 0, len(doc.Findings))
	for _, f := range doc.Findings {
		ids = append(ids, f.RuleID)
	}
	return ids
}

func TestExecuteBlockedFailsByDefault(t *testing.T) {
	labels := map[string]string{"app": "payments"}
	client := fake.NewClientset(
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "billing", Name: "payments-0", Labels: labels},
			Spec:       corev1.PodSpec{NodeName: "worker-1"},
		},
		&policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{Namespace: "billing", Name: "payments-pdb"},
			Spec:       policyv1.PodDisruptionBudgetSpec{Selector: &metav1.LabelSelector{MatchLabels: labels}},
			Status:     policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: 0},
		},
	)
	var out bytes.Buffer
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked}

	err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(client, nil), &out)

	if exitCodeOf(err) != exitFindings {
		t.Fatalf("exit code = %d (err %v), want %d", exitCodeOf(err), err, exitFindings)
	}
	if err.Error() != "" {
		t.Errorf("findings exit must be silent on stderr, got %q", err.Error())
	}
	if ids := ruleIDs(t, out.Bytes()); !slices.Contains(ids, "PDB_DISRUPTION_BLOCKED") {
		t.Errorf("rule IDs = %v, want PDB_DISRUPTION_BLOCKED", ids)
	}
}

// pdbCluster has one pod with no controller on worker-1 behind a budget that
// allows no disruption. Tests that read the exit code pass Force, so
// POD_OWNERSHIP is a RISK and the budget decides.
func pdbCluster() *fake.Clientset {
	labels := map[string]string{"app": "payments"}
	return fake.NewClientset(
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "billing", Name: "payments-0", Labels: labels},
			Spec:       corev1.PodSpec{NodeName: "worker-1"},
		},
		&policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{Namespace: "billing", Name: "payments-pdb"},
			Spec:       policyv1.PodDisruptionBudgetSpec{Selector: &metav1.LabelSelector{MatchLabels: labels}},
			Status:     policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: 0},
		},
	)
}

// kubectl drain --disable-eviction deletes the pods, so the budget that would
// refuse an eviction is not consulted and is not reported.
func TestExecuteDisableEvictionSilencesTheBudgetRules(t *testing.T) {
	for _, disable := range []bool{false, true} {
		var out bytes.Buffer
		opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked, Force: true, DisableEviction: disable}

		err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(pdbCluster(), nil), &out)

		blocked := slices.Contains(ruleIDs(t, out.Bytes()), "PDB_DISRUPTION_BLOCKED")
		if blocked == disable {
			t.Errorf("disable-eviction=%v: PDB_DISRUPTION_BLOCKED reported = %v", disable, blocked)
		}
		if want := map[bool]int{false: exitFindings, true: 0}[disable]; exitCode(err) != want {
			t.Errorf("disable-eviction=%v: exit code = %d, want %d", disable, exitCode(err), want)
		}
	}
}

// The flag shapes the result, so the log says so and how many budgets it set
// aside; without the flag there is nothing to say.
func TestExecuteLogsDisabledEviction(t *testing.T) {
	var out, logs bytes.Buffer
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked, DisableEviction: true}
	_ = execute(context.Background(), opts, debugLog(&logs), inventory.NewClientsetCollector(pdbCluster(), nil), &out)
	wantInOrder(t, logs.String(),
		`msg="resolve target nodes"`,
		`msg="eviction disabled" budgets_not_consulted=1`,
		`msg="rule start" rule=PDB_DISRUPTION_BLOCKED`)

	logs.Reset()
	opts.DisableEviction = false
	_ = execute(context.Background(), opts, debugLog(&logs), inventory.NewClientsetCollector(pdbCluster(), nil), &out)
	if strings.Contains(logs.String(), "eviction disabled") {
		t.Errorf("log mentions disabled eviction although the flag is off:\n%s", logs.String())
	}
}

// The line is logged also when the cluster has no budgets: the flag was given.
func TestExecuteLogsDisabledEvictionWithoutBudgets(t *testing.T) {
	var out, logs bytes.Buffer
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked, DisableEviction: true}
	_ = execute(context.Background(), opts, debugLog(&logs), inventory.NewClientsetCollector(unmanagedCluster(), nil), &out)
	if !strings.Contains(logs.String(), `msg="eviction disabled" budgets_not_consulted=0`) {
		t.Errorf("no eviction disabled line with 0 budgets:\n%s", logs.String())
	}
}

// --debug adds lines to stderr and nothing to stdout, also with the flag.
func TestExecuteDebugDoesNotChangeStdoutWithDisableEviction(t *testing.T) {
	for _, format := range []string{outputText, outputJSON} {
		opts := options{Nodes: []string{"worker-1"}, Output: format, FailOn: failOnRisk, DisableEviction: true}
		c := atFixedTime{inventory.NewClientsetCollector(pdbCluster(), nil)}
		var plain, debug, logs bytes.Buffer
		errPlain := execute(context.Background(), opts, nil, c, &plain)
		errDebug := execute(context.Background(), opts, debugLog(&logs), c, &debug)
		if plain.String() != debug.String() || exitCodeOf(errPlain) != exitCodeOf(errDebug) {
			t.Errorf("%s: --debug changed stdout or the exit code", format)
		}
	}
}

func TestExecuteDuplicateNodesAreAnalyzedOnce(t *testing.T) {
	var once, twice bytes.Buffer
	c := atFixedTime{inventory.NewClientsetCollector(unmanagedCluster(), nil)}

	_ = execute(context.Background(), options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked}, nil, c, &once)
	_ = execute(context.Background(), options{Nodes: []string{"worker-1", "worker-1"}, Output: outputJSON, FailOn: failOnBlocked}, nil, c, &twice)

	if once.String() != twice.String() {
		t.Errorf("duplicate --nodes changed the report:\n%s\nvs\n%s", once.String(), twice.String())
	}
}

func TestExecuteWildcardOnEmptyCluster(t *testing.T) {
	var out bytes.Buffer
	opts := options{AllNodes: true, Output: outputJSON, FailOn: failOnBlocked}

	err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(fake.NewClientset(), nil), &out)

	if err == nil || !strings.Contains(err.Error(), "no nodes") {
		t.Fatalf("error = %v, want 'no nodes'", err)
	}
	if exitCodeOf(err) != exitFailure {
		t.Errorf("exit code = %d, want %d", exitCodeOf(err), exitFailure)
	}
	if out.Len() != 0 {
		t.Errorf("wrote output on error: %s", out.String())
	}
}

func TestExecuteForbiddenIsAnError(t *testing.T) {
	client := unmanagedCluster()
	client.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("RBAC: denied"))
	})
	var out bytes.Buffer
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked}

	err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(client, nil), &out)

	if err == nil || !strings.Contains(err.Error(), "list pods") || !strings.Contains(err.Error(), "forbidden") {
		t.Fatalf("error = %v, want a forbidden error naming the pods list", err)
	}
	if !apierrors.IsForbidden(err) {
		t.Errorf("error chain lost the API status: %v", err)
	}
	if exitCodeOf(err) != exitFailure {
		t.Errorf("exit code = %d, want %d", exitCodeOf(err), exitFailure)
	}
	if out.Len() != 0 {
		t.Errorf("wrote output on error: %s", out.String())
	}
}

// ctxCollector reports the context it was called with.
type ctxCollector struct{}

func (ctxCollector) Collect(ctx context.Context) (*inventory.Snapshot, error) {
	return nil, ctx.Err()
}

func TestExecutePassesContextToCollector(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked}

	err := execute(ctx, opts, nil, ctxCollector{}, &bytes.Buffer{})

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if exitCodeOf(err) != exitFailure {
		t.Errorf("exit code = %d, want %d", exitCodeOf(err), exitFailure)
	}
}

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("broken pipe") }

func TestExecuteWriteFailure(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked}

	err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(unmanagedCluster(), nil), failingWriter{})

	if err == nil || !strings.Contains(err.Error(), "write report") {
		t.Fatalf("error = %v, want 'write report'", err)
	}
	if exitCodeOf(err) != exitFailure {
		t.Errorf("exit code = %d, want %d", exitCodeOf(err), exitFailure)
	}
}

func TestExecuteIsDeterministic(t *testing.T) {
	// Several pods across namespaces, listed by the fake in arbitrary order.
	var objs []runtime.Object
	objs = append(objs, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}})
	for _, ns := range []string{"zeta", "alpha", "mid"} {
		for _, name := range []string{"c", "a", "b"} {
			objs = append(objs, &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
				Spec:       corev1.PodSpec{NodeName: "worker-1"},
			})
		}
	}
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked, Force: true}

	var first bytes.Buffer
	if err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(fake.NewClientset(objs...), nil), &first); err != nil {
		t.Fatal(err)
	}
	if n := len(ruleIDs(t, first.Bytes())); n < 9 {
		t.Fatalf("expected at least 9 findings, got %d", n)
	}
	for i := 0; i < 5; i++ {
		var again bytes.Buffer
		if err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(fake.NewClientset(objs...), nil), &again); err != nil {
			t.Fatal(err)
		}
		if again.String() != first.String() {
			t.Fatalf("run %d differs from the first run", i+2)
		}
	}
}

// debugLog returns a logger that writes DEBUG lines in text form to buf.
func debugLog(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// wantInOrder fails unless every want appears in got, each after the previous.
func wantInOrder(t *testing.T, got string, wants ...string) {
	t.Helper()
	rest := got
	for _, w := range wants {
		i := strings.Index(rest, w)
		if i < 0 {
			t.Errorf("log is missing %q (or has it out of order):\n%s", w, got)
			return
		}
		rest = rest[i+len(w):]
	}
}

func TestExecuteLogsPipelineSteps(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnRisk, Force: true}
	var out, logs bytes.Buffer

	_ = execute(context.Background(), opts, debugLog(&logs), inventory.NewClientsetCollector(unmanagedCluster(), nil), &out)

	wantInOrder(t, logs.String(),
		`msg="resolve target nodes" requested=[worker-1] all=false targets=[worker-1]`,
		`msg="rule done" rule=POD_OWNERSHIP findings=1`,
		`msg=summary blocked=0 risk=1 unknown=0 fail_on=risk`,
		`msg="write report" format=json verbose=false findings=1`,
	)
}

func TestExecutePodSelectorLeavesUnselectedPodsOut(t *testing.T) {
	tests := []struct {
		name         string
		selector     string
		wantFindings bool
	}{
		{"no selector reports the bare pod", "", true},
		{"selector that matches nothing reports nothing", "app=web", false},
		{"negation selector matches the unlabeled pod", "app!=web", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnRisk, PodSelector: tt.selector}
			var out bytes.Buffer
			err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(unmanagedCluster(), nil), &out)
			if got := exitCodeOf(err) == exitFindings; got != tt.wantFindings {
				t.Errorf("exit code = %d (err %v), want findings = %v", exitCodeOf(err), err, tt.wantFindings)
			}
		})
	}
}

func TestExecuteRejectsAnInvalidPodSelector(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnRisk, PodSelector: "app in (web"}
	var out bytes.Buffer
	err := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(unmanagedCluster(), nil), &out)
	if err == nil || !strings.Contains(err.Error(), "--pod-selector") || exitCodeOf(err) != exitFailure {
		t.Fatalf("error = %v (exit %d), want a --pod-selector error with exit %d", err, exitCodeOf(err), exitFailure)
	}
	if out.Len() != 0 {
		t.Errorf("stdout = %q, want nothing", out.String())
	}
}

// The selector shapes the result, so the log says how many pods it kept and how
// many it left out; with no selector there is nothing to say.
func TestExecuteLogsThePodSelector(t *testing.T) {
	var out, logs bytes.Buffer
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnRisk, PodSelector: "app=web"}
	_ = execute(context.Background(), opts, debugLog(&logs), inventory.NewClientsetCollector(unmanagedCluster(), nil), &out)
	wantInOrder(t, logs.String(),
		`msg="resolve target nodes"`,
		`msg="pod selector" selector="app=web" selected=0 left_out=1`,
		`msg="rule done"`)

	logs.Reset()
	opts.PodSelector = ""
	_ = execute(context.Background(), opts, debugLog(&logs), inventory.NewClientsetCollector(unmanagedCluster(), nil), &out)
	if strings.Contains(logs.String(), "pod selector") {
		t.Errorf("log mentions a pod selector although none was given:\n%s", logs.String())
	}
}

func TestExecuteLogsRejectedTargetNodes(t *testing.T) {
	opts := options{Nodes: []string{"ghost"}, Output: outputJSON, FailOn: failOnBlocked}
	var out, logs bytes.Buffer

	_ = execute(context.Background(), opts, debugLog(&logs), inventory.NewClientsetCollector(unmanagedCluster(), nil), &out)

	wantInOrder(t, logs.String(), `msg="resolve target nodes failed"`, `requested=[ghost]`, `error="node(s) not found in cluster: ghost"`)
}

func TestExecuteDebugDoesNotChangeStdout(t *testing.T) {
	for _, format := range []string{outputText, outputJSON} {
		for _, verbose := range []bool{false, true} {
			opts := options{Nodes: []string{"worker-1"}, Output: format, Verbose: verbose, FailOn: failOnBlocked}
			c := atFixedTime{inventory.NewClientsetCollector(unmanagedCluster(), nil)}
			var plain, debug, logs bytes.Buffer

			_ = execute(context.Background(), opts, nil, c, &plain)
			_ = execute(context.Background(), opts, debugLog(&logs), c, &debug)

			if !bytes.Equal(plain.Bytes(), debug.Bytes()) {
				t.Errorf("%s verbose=%v: stdout differs with a debug logger\nwithout:\n%s\nwith:\n%s", format, verbose, plain.String(), debug.String())
			}
			if logs.Len() == 0 {
				t.Errorf("%s verbose=%v: the debug logger received nothing", format, verbose)
			}
		}
	}
}

// --debug adds lines to stderr and nothing to stdout, also when a selector
// leaves pods out.
func TestExecuteDebugDoesNotChangeStdoutWithPodSelector(t *testing.T) {
	for _, selector := range []string{"app=web", "app!=web", "   "} {
		for _, format := range []string{outputText, outputJSON} {
			opts := options{Nodes: []string{"worker-1"}, Output: format, FailOn: failOnRisk, PodSelector: selector}
			var plain, debug, logs bytes.Buffer
			errPlain := execute(context.Background(), opts, nil, inventory.NewClientsetCollector(unmanagedCluster(), nil), &plain)
			errDebug := execute(context.Background(), opts, debugLog(&logs), inventory.NewClientsetCollector(unmanagedCluster(), nil), &debug)
			if plain.String() != debug.String() || exitCodeOf(errPlain) != exitCodeOf(errDebug) {
				t.Errorf("selector %q, %s: --debug changed stdout or the exit code", selector, format)
			}
			if !strings.Contains(logs.String(), `msg="pod selector"`) {
				t.Errorf("selector %q: no pod selector line in the log:\n%s", selector, logs.String())
			}
		}
	}
}

// daemonSetCluster has one DaemonSet pod and one bare pod on worker-1.
func daemonSetCluster() *fake.Clientset {
	controller := true
	return fake.NewClientset(
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "kube-system", Name: "agent-x1", OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "apps/v1", Kind: "DaemonSet", Name: "agent", Controller: &controller}}},
			Spec: corev1.PodSpec{NodeName: "worker-1"},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "default", Name: "bare"},
			Spec:       corev1.PodSpec{NodeName: "worker-1"},
		},
	)
}

// The drain flags reach the rule: the same cluster ends the run with exit 1 or
// 0 depending on whether kubectl drain would refuse its pods.
func TestExecuteDrainFlagsDecideTheExitCode(t *testing.T) {
	tests := []struct {
		name     string
		opts     options
		wantCode int
	}{
		{"no flags: both pods are refused", options{}, exitFindings},
		{"--ignore-daemonsets alone leaves the bare pod BLOCKED", options{IgnoreDaemonSets: true}, exitFindings},
		{"--force alone leaves the DaemonSet BLOCKED", options{Force: true}, exitFindings},
		{"both flags: only a RISK is left, which passes at the blocked threshold", options{IgnoreDaemonSets: true, Force: true}, 0},
		{"--pod-selector that matches nothing: nothing is refused", options{PodSelector: "app=none"}, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.Nodes, tt.opts.Output, tt.opts.FailOn = []string{"worker-1"}, outputJSON, failOnBlocked
			var out bytes.Buffer
			err := execute(context.Background(), tt.opts, nil, inventory.NewClientsetCollector(daemonSetCluster(), nil), &out)
			got := 0
			if err != nil {
				got = exitCodeOf(err)
			}
			if got != tt.wantCode {
				t.Errorf("exit code = %d (err %v), want %d\n%s", got, err, tt.wantCode, out.String())
			}
		})
	}
}

// emptyDirCluster has one StatefulSet pod with an emptyDir volume on worker-1.
func emptyDirCluster() *fake.Clientset {
	controller := true
	return fake.NewClientset(
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker-1"}},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "db-0", Labels: map[string]string{"app": "db"}, OwnerReferences: []metav1.OwnerReference{
				{APIVersion: "apps/v1", Kind: "StatefulSet", Name: "db", Controller: &controller}}},
			Spec: corev1.PodSpec{
				NodeName: "worker-1",
				Volumes:  []corev1.Volume{{Name: "scratch", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}},
			},
		},
	)
}

// --delete-emptydir-data reaches POD_LOCAL_STORAGE: the same cluster ends the
// run with exit 1 without the flag and 0 with it, where the finding is a RISK.
func TestExecuteDeleteEmptyDirDataDecidesTheExitCode(t *testing.T) {
	tests := []struct {
		name     string
		opts     options
		wantCode int
		wantRule bool
	}{
		{"no flag: kubectl refuses the pod", options{}, exitFindings, true},
		{"--delete-emptydir-data: a RISK is left, which passes at the blocked threshold", options{DeleteEmptyDirData: true}, 0, true},
		{"--delete-emptydir-data with --fail-on risk fails again", options{DeleteEmptyDirData: true, FailOn: failOnRisk}, exitFindings, true},
		{"the other flags do not lift the refusal", options{IgnoreDaemonSets: true, Force: true, DisableEviction: true}, exitFindings, true},
		{"--pod-selector that matches nothing: nothing is refused", options{PodSelector: "app=none"}, 0, false},
		{"--pod-selector that matches the pod", options{PodSelector: "app=db"}, exitFindings, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.opts.Nodes, tt.opts.Output = []string{"worker-1"}, outputJSON
			if tt.opts.FailOn == "" {
				tt.opts.FailOn = failOnBlocked
			}
			var out bytes.Buffer
			err := execute(context.Background(), tt.opts, nil, inventory.NewClientsetCollector(emptyDirCluster(), nil), &out)
			got := 0
			if err != nil {
				got = exitCodeOf(err)
			}
			if got != tt.wantCode {
				t.Errorf("exit code = %d (err %v), want %d\n%s", got, err, tt.wantCode, out.String())
			}
			if has := slices.Contains(ruleIDs(t, out.Bytes()), "POD_LOCAL_STORAGE"); has != tt.wantRule {
				t.Errorf("POD_LOCAL_STORAGE reported = %v, want %v\n%s", has, tt.wantRule, out.String())
			}
		})
	}
}
