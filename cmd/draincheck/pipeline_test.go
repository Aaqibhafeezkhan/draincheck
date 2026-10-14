package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

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

func TestExecuteTextNotImplemented(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputText, FailOn: failOnBlocked}
	err := execute(context.Background(), opts, inventory.NewClientsetCollector(unmanagedCluster()), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "--output json") {
		t.Fatalf("error = %v, want a hint to use --output json", err)
	}
	if exitCodeOf(err) != exitFailure {
		t.Errorf("exit code = %d, want %d", exitCodeOf(err), exitFailure)
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
