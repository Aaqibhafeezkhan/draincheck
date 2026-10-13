package analyzer

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func TestMissingResourceRequests(t *testing.T) {
	s := snapshotFromFixture(t, "missing-resource-requests.yaml")
	pod := func(ns, name string) findings.ResourceRef {
		return findings.ResourceRef{Kind: "Pod", Namespace: ns, Name: name}
	}
	f := func(r findings.ResourceRef, node string, cs ...findings.MissingRequestsContainer) findings.Finding {
		return findings.Finding{
			RuleID:   "MISSING_RESOURCE_REQUESTS",
			Severity: findings.Risk,
			Resource: r,
			Evidence: findings.MissingRequestsEvidence{Node: node, Containers: cs},
		}
	}
	c := func(name string, cpu, mem bool) findings.MissingRequestsContainer {
		return findings.MissingRequestsContainer{Name: name, MissingCPU: cpu, MissingMemory: mem}
	}

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "partial and absent requests are reported; complete, pod-level, init-only, finished, DaemonSet and mirror pods are not",
			targets: []string{"worker-01"},
			want: []findings.Finding{
				f(pod("shop", "bare-0"), "worker-01", c("web", true, true)),
				f(pod("shop", "cpu-only-0"), "worker-01", c("app", false, true)),
				f(pod("shop", "mixed-0"), "worker-01", c("sidecar", true, false)),
				f(pod("tools", "debug"), "worker-01", c("debug", true, true)),
			},
		},
		{
			name:    "only pods on target nodes count",
			targets: []string{"worker-02"},
			want: []findings.Finding{
				f(pod("shop", "bare-1"), "worker-02", c("web", true, true)),
			},
		},
		{
			name:    "no targets",
			targets: nil,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Run(Input{Snapshot: s, Targets: tt.targets}, MissingResourceRequests{})

			// Ignore prose; it is covered by TestMissingResourceRequestsDescribesFinding.
			for i := range got {
				got[i].Description, got[i].Remediation = "", ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

func TestMissingResourceRequestsDescribesFinding(t *testing.T) {
	s := snapshotFromFixture(t, "missing-resource-requests.yaml")

	got := Run(Input{Snapshot: s, Targets: []string{"worker-01"}}, MissingResourceRequests{})

	if len(got) != 4 {
		t.Fatalf("got %d findings, want 4", len(got))
	}
	for _, g := range got {
		if g.Description == "" || g.Remediation == "" {
			t.Errorf("%s: description and remediation must be set: %+v", g.Resource, g)
		}
	}
	if d := got[1].Description; !strings.Contains(d, "app (memory)") {
		t.Errorf("cpu-only-0 should name container app and only memory: %s", d)
	}
}

func TestMissingResourceRequestsNilSnapshot(t *testing.T) {
	if got := (MissingResourceRequests{}).Evaluate(Input{Targets: []string{"worker-01"}}); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestMissingResourceRequestsID(t *testing.T) {
	if got := (MissingResourceRequests{}).ID(); got != "MISSING_RESOURCE_REQUESTS" {
		t.Errorf("ID() = %q", got)
	}
}
