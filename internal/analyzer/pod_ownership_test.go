package analyzer

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func TestPodOwnership(t *testing.T) {
	s := snapshotFromFixture(t, "pod-ownership.yaml")
	pod := func(ns, name string) findings.ResourceRef {
		return findings.ResourceRef{Kind: "Pod", Namespace: ns, Name: name}
	}
	f := func(r findings.ResourceRef, e findings.OwnershipEvidence) findings.Finding {
		return findings.Finding{RuleID: "POD_OWNERSHIP", Severity: findings.Risk, Resource: r, Evidence: e}
	}

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "static mirror pod, naked pod and DaemonSet pod; managed and finished pods are not reported",
			targets: []string{"worker-01"},
			want: []findings.Finding{
				f(pod("kube-system", "etcd-worker-01"), findings.OwnershipEvidence{Category: findings.OwnershipStaticPod, Node: "worker-01"}),
				f(pod("monitoring", "agent-x1"), findings.OwnershipEvidence{Category: findings.OwnershipDaemonSet, Node: "worker-01", OwnerKind: "DaemonSet", OwnerName: "agent"}),
				f(pod("tools", "debug"), findings.OwnershipEvidence{Category: findings.OwnershipUnmanaged, Node: "worker-01"}),
			},
		},
		{
			name:    "only pods on target nodes count",
			targets: []string{"worker-02"},
			want: []findings.Finding{
				f(pod("monitoring", "agent-x2"), findings.OwnershipEvidence{Category: findings.OwnershipDaemonSet, Node: "worker-02", OwnerKind: "DaemonSet", OwnerName: "agent"}),
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
			got := Run(Input{Snapshot: s, Targets: tt.targets}, PodOwnership{})

			// Ignore prose; it is covered by TestPodOwnershipDescribesFinding.
			for i := range got {
				got[i].Description, got[i].Remediation = "", ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

func TestPodOwnershipDescribesFinding(t *testing.T) {
	s := snapshotFromFixture(t, "pod-ownership.yaml")

	got := Run(Input{Snapshot: s, Targets: []string{"worker-01"}}, PodOwnership{})

	if len(got) != 3 {
		t.Fatalf("got %d findings, want 3", len(got))
	}
	wantFlag := map[string]string{
		"etcd-worker-01": "static",
		"agent-x1":       "--ignore-daemonsets",
		"debug":          "--force",
	}
	for _, g := range got {
		if g.Description == "" || g.Remediation == "" {
			t.Errorf("%s: description and remediation must be set: %+v", g.Resource, g)
		}
		if want := wantFlag[g.Resource.Name]; !strings.Contains(g.Description+g.Remediation, want) {
			t.Errorf("%s: prose should mention %q: %s / %s", g.Resource, want, g.Description, g.Remediation)
		}
	}
}

func TestPodOwnershipNilSnapshot(t *testing.T) {
	if got := (PodOwnership{}).Evaluate(Input{Targets: []string{"worker-01"}}); got != nil {
		t.Errorf("got %+v, want nil", got)
	}
}

func TestPodOwnershipID(t *testing.T) {
	if got := (PodOwnership{}).ID(); got != "POD_OWNERSHIP" {
		t.Errorf("ID() = %q", got)
	}
}
