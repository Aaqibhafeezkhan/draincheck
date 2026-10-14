package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

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

// unmanagedCluster has one bare pod on worker-1: a POD_OWNERSHIP RISK, no BLOCKED.
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
		{"risk passes at blocked threshold", options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked}, 0, true},
		{"risk fails at risk threshold", options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnRisk}, exitFindings, true},
		{"no pods on target, clean", options{Nodes: []string{"worker-2"}, Output: outputJSON, FailOn: failOnAny}, 0, false},
		{"wildcard targets every node", options{AllNodes: true, Output: outputJSON, FailOn: failOnRisk}, exitFindings, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out bytes.Buffer
			err := execute(context.Background(), tt.opts, inventory.NewClientsetCollector(unmanagedCluster()), &out)

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
	err := execute(context.Background(), opts, inventory.NewClientsetCollector(unmanagedCluster()), &out)

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

func TestExecuteTextWritesReadableReport(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputText, FailOn: failOnBlocked}
	var out bytes.Buffer
	err := execute(context.Background(), opts, inventory.NewClientsetCollector(unmanagedCluster()), &out)
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

type failingCollector struct{}

func (failingCollector) Collect(context.Context) (*inventory.Snapshot, error) {
	return nil, errors.New("boom")
}

func TestExecuteCollectError(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked}
	err := execute(context.Background(), opts, failingCollector{}, &bytes.Buffer{})
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

	err := execute(context.Background(), opts, inventory.NewClientsetCollector(client), &out)

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

func TestExecuteDuplicateNodesAreAnalyzedOnce(t *testing.T) {
	var once, twice bytes.Buffer
	c := inventory.NewClientsetCollector(unmanagedCluster())

	_ = execute(context.Background(), options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked}, c, &once)
	_ = execute(context.Background(), options{Nodes: []string{"worker-1", "worker-1"}, Output: outputJSON, FailOn: failOnBlocked}, c, &twice)

	if once.String() != twice.String() {
		t.Errorf("duplicate --nodes changed the report:\n%s\nvs\n%s", once.String(), twice.String())
	}
}

func TestExecuteWildcardOnEmptyCluster(t *testing.T) {
	var out bytes.Buffer
	opts := options{AllNodes: true, Output: outputJSON, FailOn: failOnBlocked}

	err := execute(context.Background(), opts, inventory.NewClientsetCollector(fake.NewClientset()), &out)

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

	err := execute(context.Background(), opts, inventory.NewClientsetCollector(client), &out)

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

	err := execute(ctx, opts, ctxCollector{}, &bytes.Buffer{})

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

	err := execute(context.Background(), opts, inventory.NewClientsetCollector(unmanagedCluster()), failingWriter{})

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
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnBlocked}

	var first bytes.Buffer
	if err := execute(context.Background(), opts, inventory.NewClientsetCollector(fake.NewClientset(objs...)), &first); err != nil {
		t.Fatal(err)
	}
	if n := len(ruleIDs(t, first.Bytes())); n < 9 {
		t.Fatalf("expected at least 9 findings, got %d", n)
	}
	for i := 0; i < 5; i++ {
		var again bytes.Buffer
		if err := execute(context.Background(), opts, inventory.NewClientsetCollector(fake.NewClientset(objs...)), &again); err != nil {
			t.Fatal(err)
		}
		if again.String() != first.String() {
			t.Fatalf("run %d differs from the first run", i+2)
		}
	}
}
