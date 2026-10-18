package analyzer

import (
	"reflect"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func missingContainer(name string, cpu, mem bool) findings.MissingRequestsContainer {
	return findings.MissingRequestsContainer{Name: name, MissingCPU: cpu, MissingMemory: mem}
}

func missingPod(name, node string, cs ...findings.MissingRequestsContainer) findings.MissingRequestsPod {
	return findings.MissingRequestsPod{Name: name, Node: node, Containers: cs}
}

// workloadFinding is the finding for a pod controlled by kind/ns/name. onTargets is
// the number of the workload's pods on the target nodes that kubectl drain
// would evict, with or without requests.
func workloadFinding(kind, ns, name string, onTargets int, pods ...findings.MissingRequestsPod) findings.Finding {
	return findings.Finding{
		RuleID:   "MISSING_RESOURCE_REQUESTS",
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: kind, Namespace: ns, Name: name},
		Evidence: findings.MissingRequestsEvidence{OwnerKind: kind, OwnerName: name, PodsOnTargets: onTargets, Pods: pods},
	}
}

// nakedPodFinding is the finding for a pod with no controller: the pod is its own workload.
func nakedPodFinding(ns string, pod findings.MissingRequestsPod) findings.Finding {
	return findings.Finding{
		RuleID:   "MISSING_RESOURCE_REQUESTS",
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: "Pod", Namespace: ns, Name: pod.Name},
		Evidence: findings.MissingRequestsEvidence{PodsOnTargets: 1, Pods: []findings.MissingRequestsPod{pod}},
	}
}

func TestMissingResourceRequests(t *testing.T) {
	s := snapshotFromFixture(t, "missing-resource-requests.yaml")

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "one finding per workload; complete, pod-level, init-only, finished, DaemonSet and mirror pods are not reported",
			targets: []string{"worker-01"},
			want: []findings.Finding{
				workloadFinding("Deployment", "shop", "api", 2, missingPod("api-5d4-a", "worker-01", missingContainer("app", true, true))),
				workloadFinding("Deployment", "shop", "web", 2,
					missingPod("web-7f9c-a", "worker-01", missingContainer("web", true, true)),
					missingPod("web-7f9c-b", "worker-01", missingContainer("web", true, true))),
				workloadFinding("Job", "shop", "migrate", 1, missingPod("migrate-x7k2p", "worker-01", missingContainer("job", true, true))),
				workloadFinding("StatefulSet", "shop", "bare", 1, missingPod("bare-0", "worker-01", missingContainer("web", true, true))),
				workloadFinding("StatefulSet", "shop", "cpu-only", 1, missingPod("cpu-only-0", "worker-01", missingContainer("app", false, true))),
				workloadFinding("StatefulSet", "shop", "mixed", 1, missingPod("mixed-0", "worker-01", missingContainer("sidecar", true, false))),
				nakedPodFinding("tools", missingPod("debug", "worker-01", missingContainer("debug", true, true))),
			},
		},
		{
			name:    "replicas on several target nodes are one finding with every pod and its node in evidence",
			targets: []string{"worker-01", "worker-02"},
			want: []findings.Finding{
				workloadFinding("Deployment", "shop", "api", 3,
					missingPod("api-5d4-a", "worker-01", missingContainer("app", true, true)),
					missingPod("api-5d4-c", "worker-02", missingContainer("app", true, true))),
				workloadFinding("Deployment", "shop", "web", 3,
					missingPod("web-7f9c-a", "worker-01", missingContainer("web", true, true)),
					missingPod("web-7f9c-b", "worker-01", missingContainer("web", true, true)),
					missingPod("web-7f9c-c", "worker-02", missingContainer("web", true, true))),
				workloadFinding("Job", "shop", "migrate", 1, missingPod("migrate-x7k2p", "worker-01", missingContainer("job", true, true))),
				workloadFinding("StatefulSet", "shop", "bare", 2,
					missingPod("bare-0", "worker-01", missingContainer("web", true, true)),
					missingPod("bare-1", "worker-02", missingContainer("web", true, true))),
				workloadFinding("StatefulSet", "shop", "cpu-only", 1, missingPod("cpu-only-0", "worker-01", missingContainer("app", false, true))),
				workloadFinding("StatefulSet", "shop", "mixed", 1, missingPod("mixed-0", "worker-01", missingContainer("sidecar", true, false))),
				nakedPodFinding("tools", missingPod("debug", "worker-01", missingContainer("debug", true, true))),
			},
		},
		{
			name:    "only pods on target nodes count, including in the per-workload total",
			targets: []string{"worker-02"},
			want: []findings.Finding{
				workloadFinding("Deployment", "shop", "api", 1, missingPod("api-5d4-c", "worker-02", missingContainer("app", true, true))),
				workloadFinding("Deployment", "shop", "web", 1, missingPod("web-7f9c-c", "worker-02", missingContainer("web", true, true))),
				workloadFinding("StatefulSet", "shop", "bare", 1, missingPod("bare-1", "worker-02", missingContainer("web", true, true))),
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
			got := Run(nil, Input{Snapshot: s, Targets: tt.targets}, MissingResourceRequests{})

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

// Replicas missing requests must not add up to one finding per pod: three
// replicas are one finding, and a DaemonSet pod never contributes one.
func TestMissingResourceRequestsOneFindingPerWorkload(t *testing.T) {
	s := snapshotFromFixture(t, "missing-resource-requests.yaml")

	got := Run(nil, Input{Snapshot: s, Targets: []string{"worker-01", "worker-02"}}, MissingResourceRequests{})

	var web []findings.Finding
	for _, g := range got {
		if g.Resource.Kind == "DaemonSet" || strings.Contains(g.Resource.Name, "agent") {
			t.Errorf("DaemonSet pods must not be reported: %+v", g)
		}
		if g.Resource.Kind == "Deployment" && g.Resource.Name == "web" {
			web = append(web, g)
		}
	}
	if len(web) != 1 {
		t.Fatalf("got %d findings for Deployment shop/web, want 1", len(web))
	}
	if ev := web[0].Evidence.(findings.MissingRequestsEvidence); len(ev.Pods) != 3 {
		t.Errorf("got %d pods in evidence, want 3: %+v", len(ev.Pods), ev)
	}
}

func TestMissingResourceRequestsDescribesFinding(t *testing.T) {
	s := snapshotFromFixture(t, "missing-resource-requests.yaml")

	got := Run(nil, Input{Snapshot: s, Targets: []string{"worker-01", "worker-02"}}, MissingResourceRequests{})

	byResource := make(map[string]findings.Finding, len(got))
	for _, g := range got {
		if g.Description == "" || g.Remediation == "" {
			t.Errorf("%s: description and remediation must be set: %+v", g.Resource, g)
		}
		byResource[g.Resource.String()] = g
	}

	tests := []struct {
		resource string
		contains []string
	}{
		// The count is stated once, with the total of the workload's pods on the target nodes.
		{"Deployment/shop/web", []string{"3 of 3 pods of Deployment shop/web", "web (cpu, memory)"}},
		{"Deployment/shop/api", []string{"2 of 3 pods of Deployment shop/api", "app (cpu, memory)"}},
		{"StatefulSet/shop/cpu-only", []string{"1 of 1 pod of StatefulSet shop/cpu-only", "app (memory)"}},
		{"StatefulSet/shop/mixed", []string{"sidecar (cpu)"}},
		{"Pod/tools/debug", []string{"pod tools/debug on node worker-01", "debug (cpu, memory)"}},
	}
	for _, tt := range tests {
		g, ok := byResource[tt.resource]
		if !ok {
			t.Errorf("no finding for %s", tt.resource)
			continue
		}
		for _, want := range tt.contains {
			if !strings.Contains(g.Description, want) {
				t.Errorf("%s: description %q does not contain %q", tt.resource, g.Description, want)
			}
		}
	}
}

// Every finding of the rule carries the same remediation, so the compact text
// report prints it once.
func TestMissingResourceRequestsSharesRemediation(t *testing.T) {
	s := snapshotFromFixture(t, "missing-resource-requests.yaml")

	got := Run(nil, Input{Snapshot: s, Targets: []string{"worker-01", "worker-02"}}, MissingResourceRequests{})

	for _, g := range got[1:] {
		if g.Remediation != got[0].Remediation {
			t.Errorf("%s: remediation %q differs from %q", g.Resource, g.Remediation, got[0].Remediation)
		}
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
