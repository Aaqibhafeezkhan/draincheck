package analyzer

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
)

func ownershipPod(ns, name, node string) findings.OwnershipPod {
	return findings.OwnershipPod{Namespace: ns, Name: name, Node: node}
}

func ownershipFinding(r findings.ResourceRef, e findings.OwnershipEvidence) findings.Finding {
	return findings.Finding{RuleID: "POD_OWNERSHIP", Severity: findings.Risk, Resource: r, Evidence: e}
}

// daemonSetFinding is the one finding for a DaemonSet: nodes are the nodes its
// pods run on, pods are all of its pods on the target nodes.
func daemonSetFinding(ns, name string, nodes []string, pods ...findings.OwnershipPod) findings.Finding {
	return ownershipFinding(
		findings.ResourceRef{Kind: "DaemonSet", Namespace: ns, Name: name},
		findings.OwnershipEvidence{Category: findings.OwnershipDaemonSet, OwnerKind: "DaemonSet", OwnerName: name, Nodes: nodes, Pods: pods})
}

// staticPodsFinding is the one finding for the static pods of a node.
func staticPodsFinding(node string, pods ...findings.OwnershipPod) findings.Finding {
	return ownershipFinding(
		findings.ResourceRef{Kind: "Node", Name: node},
		findings.OwnershipEvidence{Category: findings.OwnershipStaticPod, Nodes: []string{node}, Pods: pods})
}

// unmanagedFinding is the finding for a pod with no controller.
func unmanagedFinding(pod findings.OwnershipPod) findings.Finding {
	return ownershipFinding(
		findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name},
		findings.OwnershipEvidence{Category: findings.OwnershipUnmanaged, Nodes: []string{pod.Node}, Pods: []findings.OwnershipPod{pod}})
}

func TestPodOwnership(t *testing.T) {
	s := snapshotFromFixture(t, "pod-ownership.yaml")

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "one finding per DaemonSet, per node with static pods and per naked pod; managed and finished pods are not reported",
			targets: []string{"worker-01"},
			want: []findings.Finding{
				staticPodsFinding("worker-01",
					ownershipPod("edge", "proxy-worker-01", "worker-01"),
					ownershipPod("kube-system", "etcd-worker-01", "worker-01")),
				daemonSetFinding("logging", "agent", []string{"worker-01"}, ownershipPod("logging", "agent-l1", "worker-01")),
				daemonSetFinding("monitoring", "agent", []string{"worker-01"}, ownershipPod("monitoring", "agent-x1", "worker-01")),
				unmanagedFinding(ownershipPod("tools", "debug", "worker-01")),
			},
		},
		{
			name:    "a DaemonSet on three nodes is one finding with three pods and three nodes; static pods stay one finding per node",
			targets: []string{"worker-01", "worker-02", "worker-03"},
			want: []findings.Finding{
				staticPodsFinding("worker-01",
					ownershipPod("edge", "proxy-worker-01", "worker-01"),
					ownershipPod("kube-system", "etcd-worker-01", "worker-01")),
				staticPodsFinding("worker-02", ownershipPod("kube-system", "etcd-worker-02", "worker-02")),
				daemonSetFinding("logging", "agent", []string{"worker-01"}, ownershipPod("logging", "agent-l1", "worker-01")),
				daemonSetFinding("monitoring", "agent", []string{"worker-01", "worker-02", "worker-03"},
					ownershipPod("monitoring", "agent-x1", "worker-01"),
					ownershipPod("monitoring", "agent-x2", "worker-02"),
					ownershipPod("monitoring", "agent-x3", "worker-03")),
				unmanagedFinding(ownershipPod("tools", "debug", "worker-01")),
				unmanagedFinding(ownershipPod("tools", "scratch", "worker-02")),
			},
		},
		{
			name:    "only pods on target nodes count",
			targets: []string{"worker-02"},
			want: []findings.Finding{
				staticPodsFinding("worker-02", ownershipPod("kube-system", "etcd-worker-02", "worker-02")),
				daemonSetFinding("monitoring", "agent", []string{"worker-02"}, ownershipPod("monitoring", "agent-x2", "worker-02")),
				unmanagedFinding(ownershipPod("tools", "scratch", "worker-02")),
			},
		},
		{
			name:    "a node with only DaemonSet pods",
			targets: []string{"worker-03"},
			want: []findings.Finding{
				daemonSetFinding("monitoring", "agent", []string{"worker-03"}, ownershipPod("monitoring", "agent-x3", "worker-03")),
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
			got := Run(nil, Input{Snapshot: s, Targets: tt.targets}, PodOwnership{})

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

// Grouping changes how the facts are presented, not whether they exist: every
// pod the rule reports for the targets must be in the evidence of exactly one
// finding.
func TestPodOwnershipKeepsEveryPod(t *testing.T) {
	s := snapshotFromFixture(t, "pod-ownership.yaml")

	got := Run(nil, Input{Snapshot: s, Targets: []string{"worker-01", "worker-02", "worker-03"}}, PodOwnership{})

	seen := make(map[findings.OwnershipPod]int)
	for _, g := range got {
		for _, pod := range g.Evidence.(findings.OwnershipEvidence).Pods {
			seen[pod]++
		}
	}
	want := []findings.OwnershipPod{
		ownershipPod("monitoring", "agent-x1", "worker-01"),
		ownershipPod("monitoring", "agent-x2", "worker-02"),
		ownershipPod("monitoring", "agent-x3", "worker-03"),
		ownershipPod("logging", "agent-l1", "worker-01"),
		ownershipPod("kube-system", "etcd-worker-01", "worker-01"),
		ownershipPod("edge", "proxy-worker-01", "worker-01"),
		ownershipPod("kube-system", "etcd-worker-02", "worker-02"),
		ownershipPod("tools", "debug", "worker-01"),
		ownershipPod("tools", "scratch", "worker-02"),
	}
	for _, pod := range want {
		if seen[pod] != 1 {
			t.Errorf("pod %s/%s on %s is in the evidence of %d findings, want 1", pod.Namespace, pod.Name, pod.Node, seen[pod])
		}
	}
	if len(seen) != len(want) {
		t.Errorf("evidence lists %d distinct pods, want %d: %v", len(seen), len(want), seen)
	}
}

// The same cluster must give byte-identical findings whatever order the
// snapshot lists its pods in, including the order of pods and nodes inside the
// evidence.
func TestPodOwnershipIgnoresPodOrder(t *testing.T) {
	s := snapshotFromFixture(t, "pod-ownership.yaml")
	pods := s.Pods()
	slices.Reverse(pods)
	reversed := inventory.NewSnapshot(s.Nodes(), pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())
	targets := []string{"worker-01", "worker-02", "worker-03"}

	want := Run(nil, Input{Snapshot: s, Targets: targets}, PodOwnership{})
	got := Run(nil, Input{Snapshot: reversed, Targets: targets}, PodOwnership{})

	if !reflect.DeepEqual(got, want) {
		t.Errorf("reversing the pod order changed the findings\n got: %+v\nwant: %+v", got, want)
	}
}

func TestPodOwnershipDescribesFinding(t *testing.T) {
	s := snapshotFromFixture(t, "pod-ownership.yaml")

	got := Run(nil, Input{Snapshot: s, Targets: []string{"worker-01", "worker-02", "worker-03"}}, PodOwnership{})

	byResource := make(map[string]findings.Finding, len(got))
	for _, g := range got {
		if g.Description == "" || g.Remediation == "" {
			t.Errorf("%s: description and remediation must be set: %+v", g.Resource, g)
		}
		byResource[g.Resource.String()] = g
	}

	tests := []struct {
		resource    string
		description []string
		remediation []string
	}{
		{
			resource:    "DaemonSet/monitoring/agent",
			description: []string{"DaemonSet monitoring/agent has 3 pods on 3 target nodes", "--ignore-daemonsets"},
			remediation: []string{"--ignore-daemonsets"},
		},
		{
			resource:    "DaemonSet/logging/agent",
			description: []string{"DaemonSet logging/agent has 1 pod on 1 target node"},
			remediation: []string{"--ignore-daemonsets"},
		},
		{
			resource:    "Node/worker-01",
			description: []string{"Node worker-01 runs 2 static pods", "edge/proxy-worker-01", "kube-system/etcd-worker-01", "stop when the node goes down"},
			remediation: []string{"kubectl drain skips them", "node is down"},
		},
		{
			resource:    "Node/worker-02",
			description: []string{"Node worker-02 runs 1 static pod", "kube-system/etcd-worker-02"},
			remediation: []string{"kubectl drain skips them"},
		},
		{
			resource:    "Pod/tools/debug",
			description: []string{"Pod tools/debug on node worker-01 has no controller", "--force"},
			remediation: []string{"--force"},
		},
	}
	for _, tt := range tests {
		g, ok := byResource[tt.resource]
		if !ok {
			t.Errorf("no finding for %s", tt.resource)
			continue
		}
		for _, want := range tt.description {
			if !strings.Contains(g.Description, want) {
				t.Errorf("%s: description %q does not contain %q", tt.resource, g.Description, want)
			}
		}
		for _, want := range tt.remediation {
			if !strings.Contains(g.Remediation, want) {
				t.Errorf("%s: remediation %q does not contain %q", tt.resource, g.Remediation, want)
			}
		}
	}

	// Findings with the same cause share one remediation, so the compact text
	// report prints it once.
	if a, b := byResource["DaemonSet/monitoring/agent"], byResource["DaemonSet/logging/agent"]; a.Remediation != b.Remediation {
		t.Errorf("DaemonSet findings differ in remediation: %q vs %q", a.Remediation, b.Remediation)
	}
	if a, b := byResource["Node/worker-01"], byResource["Node/worker-02"]; a.Remediation != b.Remediation {
		t.Errorf("static pod findings differ in remediation: %q vs %q", a.Remediation, b.Remediation)
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

// With --pod-selector, a pod the selector does not match is not reported, and
// the evidence of a finding lists only the pods the drain acts on.
func TestPodOwnershipHonoursPodSelector(t *testing.T) {
	mk := func(name string, spec drainPodSpec, labels map[string]string) corev1.Pod {
		pod := spec.pod()
		pod.Namespace, pod.Name, pod.Labels = "shop", name, labels
		pod.Spec.NodeName = "worker-01"
		if spec.controllerKind == "DaemonSet" {
			pod.OwnerReferences[0].APIVersion = "apps/v1"
			pod.OwnerReferences[0].Name = "agent"
		}
		return pod
	}
	web, db := map[string]string{"app": "web"}, map[string]string{"app": "db"}
	running := corev1.PodRunning
	pods := []corev1.Pod{
		mk("bare-web", drainPodSpec{phase: running}, web),
		mk("bare-db", drainPodSpec{phase: running}, db),
		mk("agent-web", drainPodSpec{controllerKind: "DaemonSet", phase: running}, web),
		mk("agent-db", drainPodSpec{controllerKind: "DaemonSet", phase: running}, db),
		mk("static-db", drainPodSpec{mirror: true, nodeOwner: true, phase: running}, db),
	}
	s := inventory.NewSnapshot(nil, pods, nil, nil, time.Time{})
	run := func(selector string) []findings.Finding {
		drain := DrainOptions{}
		if selector != "" {
			drain.PodSelector = mustSelector(t, selector)
		}
		return PodOwnership{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}, Drain: drain})
	}
	podNames := func(fs []findings.Finding) []string {
		var names []string
		for _, f := range fs {
			for _, p := range f.Evidence.(findings.OwnershipEvidence).Pods {
				names = append(names, p.Name)
			}
		}
		slices.Sort(names)
		return names
	}

	if got, want := podNames(run("")), []string{"agent-db", "agent-web", "bare-db", "bare-web", "static-db"}; !slices.Equal(got, want) {
		t.Errorf("no selector: pods = %v, want %v", got, want)
	}
	if got, want := podNames(run("app=web")), []string{"agent-web", "bare-web"}; !slices.Equal(got, want) {
		t.Errorf("app=web: pods = %v, want %v", got, want)
	}
	if got, want := podNames(run("app=db")), []string{"agent-db", "bare-db", "static-db"}; !slices.Equal(got, want) {
		t.Errorf("app=db: pods = %v, want %v", got, want)
	}
	if got := run("app=none"); len(got) != 0 {
		t.Errorf("a selector that matches nothing: findings = %v, want none", got)
	}
}
