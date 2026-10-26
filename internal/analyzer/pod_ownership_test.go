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

func ownershipFinding(sev findings.Severity, r findings.ResourceRef, e findings.OwnershipEvidence) findings.Finding {
	return findings.Finding{RuleID: "POD_OWNERSHIP", Severity: sev, Resource: r, Evidence: e}
}

// daemonSetFinding is the one finding for a DaemonSet: nodes are the nodes its
// pods run on, pods are all of its pods on the target nodes. Without
// --ignore-daemonsets kubectl drain refuses these pods, so it is BLOCKED.
func daemonSetFinding(ns, name string, nodes []string, pods ...findings.OwnershipPod) findings.Finding {
	return ownershipFinding(findings.Blocked,
		findings.ResourceRef{Kind: "DaemonSet", Namespace: ns, Name: name},
		findings.OwnershipEvidence{Category: findings.OwnershipDaemonSet, OwnerKind: "DaemonSet", OwnerName: name, Nodes: nodes, Pods: pods})
}

// staticPodsFinding is the one finding for the static pods of a node.
func staticPodsFinding(node string, pods ...findings.OwnershipPod) findings.Finding {
	return ownershipFinding(findings.Risk,
		findings.ResourceRef{Kind: "Node", Name: node},
		findings.OwnershipEvidence{Category: findings.OwnershipStaticPod, Nodes: []string{node}, Pods: pods})
}

// unmanagedFinding is the finding for a pod with no controller. Without
// --force kubectl drain refuses it, so it is BLOCKED.
func unmanagedFinding(pod findings.OwnershipPod) findings.Finding {
	return ownershipFinding(findings.Blocked,
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

// ownershipCase is one pod of the flag matrix. Every pod runs on worker-01.
type ownershipCase struct {
	pod drainPodSpec
	// label is set on the pod, to try --pod-selector.
	label string
}

func (c ownershipCase) build() corev1.Pod {
	pod := c.pod.pod()
	pod.Name = "p"
	pod.Spec.NodeName = "worker-01"
	if c.pod.controllerKind == "DaemonSet" {
		pod.OwnerReferences[0].APIVersion = "apps/v1"
		pod.OwnerReferences[0].Name = "agent"
	}
	if c.label != "" {
		pod.Labels = map[string]string{"app": c.label}
	}
	return pod
}

// severityOf runs the rule on one pod and returns the severity of its single
// finding, or "" when there is none. It fails on more than one finding.
func severityOf(t *testing.T, pod corev1.Pod, drain DrainOptions) findings.Severity {
	t.Helper()
	s := inventory.NewSnapshot(nil, []corev1.Pod{pod}, nil, nil, time.Time{})
	got := PodOwnership{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}, Drain: drain})
	switch len(got) {
	case 0:
		return ""
	case 1:
		return got[0].Severity
	}
	t.Fatalf("one pod gave %d findings: %+v", len(got), got)
	return ""
}

// kubectl drain refuses a DaemonSet pod without --ignore-daemonsets and a pod
// with no controller without --force. The finding follows: BLOCKED while the
// drain would stop, nothing once kubectl skips the pod, RISK once the flag lets
// kubectl delete a pod that nothing recreates. A static pod is skipped whatever
// the flags are, so it stays RISK.
func TestPodOwnershipSeverityFollowsFlags(t *testing.T) {
	running := corev1.PodRunning
	daemonSet := drainPodSpec{controllerKind: "DaemonSet", phase: running}
	naked := drainPodSpec{phase: running}
	static := drainPodSpec{mirror: true, nodeOwner: true, phase: running}

	tests := []struct {
		name  string
		pod   drainPodSpec
		drain DrainOptions
		want  findings.Severity
	}{
		{"DaemonSet, no flags", daemonSet, DrainOptions{}, findings.Blocked},
		{"DaemonSet, --ignore-daemonsets", daemonSet, DrainOptions{IgnoreDaemonSets: true}, ""},
		{"DaemonSet, --force does not replace --ignore-daemonsets", daemonSet, DrainOptions{Force: true}, findings.Blocked},
		{"DaemonSet, --delete-emptydir-data is unrelated", daemonSet, DrainOptions{DeleteEmptyDirData: true}, findings.Blocked},
		{"DaemonSet, --disable-eviction is unrelated", daemonSet, DrainOptions{DisableEviction: true}, findings.Blocked},
		{"DaemonSet, all flags", daemonSet, DrainOptions{IgnoreDaemonSets: true, Force: true, DeleteEmptyDirData: true}, ""},
		{"no controller, no flags", naked, DrainOptions{}, findings.Blocked},
		{"no controller, --force", naked, DrainOptions{Force: true}, findings.Risk},
		{"no controller, --ignore-daemonsets does not help", naked, DrainOptions{IgnoreDaemonSets: true}, findings.Blocked},
		{"no controller, --delete-emptydir-data does not help", naked, DrainOptions{DeleteEmptyDirData: true}, findings.Blocked},
		{"no controller, all flags", naked, DrainOptions{IgnoreDaemonSets: true, Force: true, DeleteEmptyDirData: true}, findings.Risk},
		{"static pod, no flags", static, DrainOptions{}, findings.Risk},
		{"static pod, all flags", static, DrainOptions{IgnoreDaemonSets: true, Force: true, DeleteEmptyDirData: true}, findings.Risk},

		// The class decides, as before: a terminating pod is still judged.
		{"terminating DaemonSet pod, no flags", drainPodSpec{controllerKind: "DaemonSet", phase: running, terminating: true}, DrainOptions{}, findings.Blocked},
		{"terminating DaemonSet pod, --ignore-daemonsets", drainPodSpec{controllerKind: "DaemonSet", phase: running, terminating: true}, DrainOptions{IgnoreDaemonSets: true}, ""},
		{"terminating pod with no controller, no flags", drainPodSpec{phase: running, terminating: true}, DrainOptions{}, findings.Blocked},
		{"terminating pod with no controller, --force", drainPodSpec{phase: running, terminating: true}, DrainOptions{Force: true}, findings.Risk},

		// Pending and Unknown pods are judged like Running ones.
		{"Pending pod with no controller, no flags", drainPodSpec{phase: corev1.PodPending}, DrainOptions{}, findings.Blocked},
		{"Unknown-phase DaemonSet pod, no flags", drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodUnknown}, DrainOptions{}, findings.Blocked},
		{"pod with an empty phase and no controller", drainPodSpec{}, DrainOptions{}, findings.Blocked},

		// A finished pod is removed without checks, whatever the flags.
		{"Succeeded pod with no controller", drainPodSpec{phase: corev1.PodSucceeded}, DrainOptions{}, ""},
		{"Failed DaemonSet pod", drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodFailed}, DrainOptions{}, ""},
		{"Failed static pod", drainPodSpec{mirror: true, nodeOwner: true, phase: corev1.PodFailed}, DrainOptions{}, ""},

		// An owner that is not the controller does not count as one.
		{"owner reference that is not the controller, no flags", drainPodSpec{controllerKind: "ReplicaSet", notController: true, phase: running}, DrainOptions{}, findings.Blocked},
		{"owner reference that is not the controller, --force", drainPodSpec{controllerKind: "ReplicaSet", notController: true, phase: running}, DrainOptions{Force: true}, findings.Risk},

		// A pod managed by anything else is never reported.
		{"ReplicaSet pod", drainPodSpec{controllerKind: "ReplicaSet", phase: running}, DrainOptions{}, ""},
		{"Job pod", drainPodSpec{controllerKind: "Job", phase: running}, DrainOptions{Force: true}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := ownershipCase{pod: tt.pod}.build()
			if got := severityOf(t, pod, tt.drain); got != tt.want {
				t.Errorf("severity = %q, want %q", got, tt.want)
			}
		})
	}
}

// Edge cases where the pod filters and the class could disagree. The finding
// follows kubectl's filter.
func TestPodOwnershipEdgeCases(t *testing.T) {
	t.Run("an owner of kind DaemonSet in another API group is not a DaemonSet to kubectl, so it is not refused", func(t *testing.T) {
		pod := ownershipCase{pod: drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodRunning}}.build()
		pod.OwnerReferences[0].APIVersion = "example.com/v1"
		if got := severityOf(t, pod, DrainOptions{}); got != "" {
			t.Errorf("severity = %q, want none", got)
		}
	})
	t.Run("a pod with an emptyDir volume and no controller is judged by the controller check only", func(t *testing.T) {
		// kubectl names the emptyDir first, but that is the job of a rule of its
		// own (#29); POD_OWNERSHIP reports the missing controller.
		pod := ownershipCase{pod: drainPodSpec{phase: corev1.PodRunning}}.build()
		pod.Spec.Volumes = []corev1.Volume{{Name: "scratch", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
		if got := severityOf(t, pod, DrainOptions{Force: true}); got != findings.Risk {
			t.Errorf("--force: severity = %q, want RISK", got)
		}
		if got := severityOf(t, pod, DrainOptions{}); got != findings.Blocked {
			t.Errorf("no flags: severity = %q, want BLOCKED", got)
		}
	})
	t.Run("a static pod with a DaemonSet owner stays a static pod finding", func(t *testing.T) {
		pod := ownershipCase{pod: drainPodSpec{controllerKind: "DaemonSet", mirror: true, phase: corev1.PodRunning}}.build()
		pod.OwnerReferences[0].APIVersion = "apps/v1"
		s := inventory.NewSnapshot(nil, []corev1.Pod{pod}, nil, nil, time.Time{})
		got := PodOwnership{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}})
		if len(got) != 1 || got[0].Evidence.(findings.OwnershipEvidence).Category != findings.OwnershipStaticPod || got[0].Severity != findings.Risk {
			t.Errorf("got %+v, want one RISK static pod finding", got)
		}
	})
	t.Run("a pod on a node that is not a target is never judged", func(t *testing.T) {
		pod := ownershipCase{pod: drainPodSpec{phase: corev1.PodRunning}}.build()
		pod.Spec.NodeName = "worker-02"
		if got := severityOf(t, pod, DrainOptions{}); got != "" {
			t.Errorf("severity = %q, want none", got)
		}
	})
	t.Run("a pod that --pod-selector does not match is not judged, with or without a flag", func(t *testing.T) {
		pod := ownershipCase{pod: drainPodSpec{phase: corev1.PodRunning}, label: "db"}.build()
		for _, drain := range []DrainOptions{{PodSelector: mustSelector(t, "app=web")}, {PodSelector: mustSelector(t, "app=web"), Force: true}} {
			if got := severityOf(t, pod, drain); got != "" {
				t.Errorf("%+v: severity = %q, want none", drain, got)
			}
		}
	})
}

// The flags apply to the whole drain, so one run can hold findings of both
// severities, and a flag removes only the findings it covers.
func TestPodOwnershipFlagsInOneRun(t *testing.T) {
	s := snapshotFromFixture(t, "pod-ownership.yaml")
	targets := []string{"worker-01", "worker-02", "worker-03"}
	bySeverity := func(drain DrainOptions) map[string]findings.Severity {
		out := map[string]findings.Severity{}
		for _, f := range Run(nil, Input{Snapshot: s, Targets: targets, Drain: drain}, PodOwnership{}) {
			out[f.Resource.String()] = f.Severity
		}
		return out
	}

	tests := []struct {
		name  string
		drain DrainOptions
		want  map[string]findings.Severity
	}{
		{"no flags", DrainOptions{}, map[string]findings.Severity{
			"DaemonSet/monitoring/agent": findings.Blocked, "DaemonSet/logging/agent": findings.Blocked,
			"Node/worker-01": findings.Risk, "Node/worker-02": findings.Risk,
			"Pod/tools/debug": findings.Blocked, "Pod/tools/scratch": findings.Blocked,
		}},
		{"--ignore-daemonsets removes the DaemonSet findings only", DrainOptions{IgnoreDaemonSets: true}, map[string]findings.Severity{
			"Node/worker-01": findings.Risk, "Node/worker-02": findings.Risk,
			"Pod/tools/debug": findings.Blocked, "Pod/tools/scratch": findings.Blocked,
		}},
		{"--force turns the naked pods to RISK and leaves the DaemonSets BLOCKED", DrainOptions{Force: true}, map[string]findings.Severity{
			"DaemonSet/monitoring/agent": findings.Blocked, "DaemonSet/logging/agent": findings.Blocked,
			"Node/worker-01": findings.Risk, "Node/worker-02": findings.Risk,
			"Pod/tools/debug": findings.Risk, "Pod/tools/scratch": findings.Risk,
		}},
		{"both flags", DrainOptions{IgnoreDaemonSets: true, Force: true}, map[string]findings.Severity{
			"Node/worker-01": findings.Risk, "Node/worker-02": findings.Risk,
			"Pod/tools/debug": findings.Risk, "Pod/tools/scratch": findings.Risk,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := bySeverity(tt.drain); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("severities mismatch\n got: %v\nwant: %v", got, tt.want)
			}
		})
	}
}

// A flag changes the verdict, not the facts: with the flag that makes a pod
// harmless, the pods of the findings that remain are still all listed.
func TestPodOwnershipEvidenceDoesNotDependOnFlags(t *testing.T) {
	s := snapshotFromFixture(t, "pod-ownership.yaml")
	targets := []string{"worker-01", "worker-02", "worker-03"}
	evidence := func(drain DrainOptions) map[string]findings.OwnershipEvidence {
		out := map[string]findings.OwnershipEvidence{}
		for _, f := range Run(nil, Input{Snapshot: s, Targets: targets, Drain: drain}, PodOwnership{}) {
			out[f.Resource.String()] = f.Evidence.(findings.OwnershipEvidence)
		}
		return out
	}
	plain, forced := evidence(DrainOptions{}), evidence(DrainOptions{Force: true, IgnoreDaemonSets: true})
	for resource, ev := range forced {
		if !reflect.DeepEqual(ev, plain[resource]) {
			t.Errorf("%s: evidence changed with the flags\n got: %+v\nwant: %+v", resource, ev, plain[resource])
		}
	}
}

// Each severity has its own words: the fact (what kubectl does with the pod
// under these flags), and an action that fits it.
func TestPodOwnershipDescribesSeverity(t *testing.T) {
	s := snapshotFromFixture(t, "pod-ownership.yaml")
	targets := []string{"worker-01", "worker-02", "worker-03"}
	byResource := func(drain DrainOptions) map[string]findings.Finding {
		out := map[string]findings.Finding{}
		for _, f := range Run(nil, Input{Snapshot: s, Targets: targets, Drain: drain}, PodOwnership{}) {
			out[f.Resource.String()] = f
		}
		return out
	}
	contains := func(t *testing.T, what, text string, wants ...string) {
		t.Helper()
		for _, w := range wants {
			if !strings.Contains(text, w) {
				t.Errorf("%s %q does not contain %q", what, text, w)
			}
		}
	}

	t.Run("BLOCKED naked pod says kubectl refuses it and offers --force", func(t *testing.T) {
		f := byResource(DrainOptions{})["Pod/tools/debug"]
		contains(t, "description", f.Description, "no controller", "refuses", "--force")
		contains(t, "remediation", f.Remediation, "--force", "kubectl get pod")
	})
	t.Run("RISK naked pod with --force says it is deleted and not recreated, and does not ask for --force", func(t *testing.T) {
		f := byResource(DrainOptions{Force: true})["Pod/tools/debug"]
		contains(t, "description", f.Description, "no controller", "--force", "deletes", "not be recreated")
		if strings.Contains(f.Description, "refuses") {
			t.Errorf("description %q says kubectl refuses, but --force is set", f.Description)
		}
		if strings.Contains(f.Remediation, "drain with --force") {
			t.Errorf("remediation %q asks for a flag that is already set", f.Remediation)
		}
		contains(t, "remediation", f.Remediation, "kubectl get pod", "controller")
	})
	t.Run("BLOCKED DaemonSet says kubectl refuses it and offers --ignore-daemonsets", func(t *testing.T) {
		f := byResource(DrainOptions{})["DaemonSet/monitoring/agent"]
		contains(t, "description", f.Description, "refuses", "--ignore-daemonsets")
		contains(t, "remediation", f.Remediation, "--ignore-daemonsets", "kubectl get daemonset")
	})
	t.Run("a static pod keeps its text whatever the flags are", func(t *testing.T) {
		a, b := byResource(DrainOptions{})["Node/worker-01"], byResource(DrainOptions{Force: true, IgnoreDaemonSets: true})["Node/worker-01"]
		if a.Description != b.Description || a.Remediation != b.Remediation {
			t.Errorf("static pod text changed with the flags:\n%+v\n%+v", a, b)
		}
	})
	t.Run("findings of one cause and severity share their remediation", func(t *testing.T) {
		got := byResource(DrainOptions{Force: true})
		if a, b := got["Pod/tools/debug"], got["Pod/tools/scratch"]; a.Remediation != b.Remediation {
			t.Errorf("remediation differs: %q vs %q", a.Remediation, b.Remediation)
		}
	})
}
