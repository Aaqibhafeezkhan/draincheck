package analyzer

import (
	"testing"
	"time"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var stuckNow = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// stuckPod is a Running pod on worker-01 whose deadline (deletionTimestamp, the
// time deletion started plus the grace period) was deadlineAgo before stuckNow.
func stuckPod(name string, spec drainPodSpec, deadlineAgo time.Duration, finalizers ...string) corev1.Pod {
	spec.phase = corev1.PodRunning
	p := spec.pod()
	p.Name = name
	p.Spec.NodeName = "worker-01"
	ts := metav1.NewTime(stuckNow.Add(-deadlineAgo))
	p.DeletionTimestamp = &ts
	grace := int64(30)
	p.DeletionGracePeriodSeconds = &grace
	p.Finalizers = finalizers
	return p
}

func stuckSnapshot(pods ...corev1.Pod) *inventory.Snapshot {
	return inventory.NewSnapshot(nil, pods, nil, nil, stuckNow)
}

func TestPodStuckTerminating(t *testing.T) {
	rs := drainPodSpec{controllerKind: "ReplicaSet"}
	tests := []struct {
		name    string
		pod     corev1.Pod
		targets []string
		drain   DrainOptions
		want    bool
	}{
		{"past the deadline and the margin", stuckPod("a", rs, stuckTerminatingMargin+time.Second), []string{"worker-01"}, DrainOptions{}, true},
		{"exactly at the margin is not yet stuck", stuckPod("a", rs, stuckTerminatingMargin), []string{"worker-01"}, DrainOptions{}, false},
		{"inside the grace period (deadline ahead)", stuckPod("a", rs, -20*time.Second), []string{"worker-01"}, DrainOptions{}, false},
		{"past the deadline but inside the margin", stuckPod("a", rs, stuckTerminatingMargin/2), []string{"worker-01"}, DrainOptions{}, false},
		{"not on a target node", stuckPod("a", rs, time.Hour), []string{"worker-02"}, DrainOptions{}, false},
		{"a DaemonSet pod is skipped by drain", stuckPod("a", drainPodSpec{controllerKind: "DaemonSet"}, time.Hour), []string{"worker-01"}, DrainOptions{}, false},
		{"a static pod is skipped by drain", stuckPod("a", drainPodSpec{mirror: true}, time.Hour), []string{"worker-01"}, DrainOptions{}, false},
		// Observed: a Completed pod held by a finalizer made kubectl drain wait until --timeout.
		{"a finished pod is deleted and waited for", func() corev1.Pod {
			p := stuckPod("a", rs, time.Hour, "example.com/hold")
			p.Status.Phase = corev1.PodSucceeded
			return p
		}(), []string{"worker-01"}, DrainOptions{}, true},
		{"a finished DaemonSet pod is waited for (observed)", func() corev1.Pod {
			p := stuckPod("a", drainPodSpec{controllerKind: "DaemonSet"}, time.Hour, "example.com/hold")
			p.Status.Phase = corev1.PodFailed
			return p
		}(), []string{"worker-01"}, DrainOptions{}, true},
		{"a finished static pod is still skipped", func() corev1.Pod {
			p := stuckPod("a", drainPodSpec{mirror: true}, time.Hour)
			p.Status.Phase = corev1.PodFailed
			return p
		}(), []string{"worker-01"}, DrainOptions{}, false},
		{"a pod --pod-selector leaves out", stuckPod("a", rs, time.Hour), []string{"worker-01"}, DrainOptions{PodSelector: mustSelector(t, "app=other")}, false},
		{"a controller-less pod is reported too", stuckPod("a", drainPodSpec{}, time.Hour), []string{"worker-01"}, DrainOptions{}, true},
		{"a pod that is not terminating", func() corev1.Pod {
			p := stuckPod("a", rs, time.Hour)
			p.DeletionTimestamp = nil
			return p
		}(), []string{"worker-01"}, DrainOptions{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PodStuckTerminating{}.Evaluate(Input{Snapshot: stuckSnapshot(tt.pod), Targets: tt.targets, Drain: tt.drain})
			if (len(got) == 1) != tt.want || len(got) > 1 {
				t.Fatalf("want a finding: %v, got %+v", tt.want, got)
			}
		})
	}
}

func TestPodStuckTerminatingFinding(t *testing.T) {
	rs := drainPodSpec{controllerKind: "ReplicaSet"}
	held := stuckPod("held", rs, 5*time.Minute, "example.com/hold", "other/hold")
	free := stuckPod("free", rs, 5*time.Minute)
	got := PodStuckTerminating{}.Evaluate(Input{Snapshot: stuckSnapshot(held, free), Targets: []string{"worker-01"}})
	findings.Sort(got)
	if len(got) != 2 {
		t.Fatalf("want 2 findings, got %+v", got)
	}
	f, nf := got[1], got[0] // free sorts before held
	if f.RuleID != PodStuckTerminatingID || f.Severity != findings.Risk || f.Resource != (findings.ResourceRef{Kind: "Pod", Namespace: "shop", Name: "held"}) {
		t.Errorf("unexpected header: %+v", f)
	}
	want := findings.StuckTerminatingEvidence{
		Node:               "worker-01",
		Deadline:           stuckNow.Add(-5 * time.Minute),
		GracePeriodSeconds: 30,
		Finalizers:         []string{"example.com/hold", "other/hold"},
	}
	ev, ok := f.Evidence.(findings.StuckTerminatingEvidence)
	if !ok || !ev.Deadline.Equal(want.Deadline) || ev.Node != want.Node || ev.GracePeriodSeconds != 30 || len(ev.Finalizers) != 2 {
		t.Errorf("evidence = %+v, want %+v", f.Evidence, want)
	}
	if f.Case != podStuckTerminatingFinalizer || nf.Case != podStuckTerminatingNoFinalizer {
		t.Errorf("cases = %q, %q", f.Case, nf.Case)
	}
	// JSON carries [] and not null for a pod without finalizers.
	if nev := nf.Evidence.(findings.StuckTerminatingEvidence); nev.Finalizers == nil {
		t.Error("finalizers is nil; the JSON would say null")
	}
}

// The same snapshot gives the same findings: the clock is the snapshot's.
func TestPodStuckTerminatingUsesSnapshotTime(t *testing.T) {
	p := stuckPod("a", drainPodSpec{controllerKind: "ReplicaSet"}, time.Hour)
	early := inventory.NewSnapshot(nil, []corev1.Pod{p}, nil, nil, stuckNow.Add(-2*time.Hour))
	if got := (PodStuckTerminating{}).Evaluate(Input{Snapshot: early, Targets: []string{"worker-01"}}); len(got) != 0 {
		t.Fatalf("a snapshot taken before the deadline reported %+v", got)
	}
}

// testdata/analyzer/pod-stuck-terminating.yaml: X
func TestPodStuckTerminatingFixture(t *testing.T) {
	s := snapshotFromFixture(t, "pod-stuck-terminating.yaml")
	got := PodStuckTerminating{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}})
	cases := map[string]string{}
	for _, f := range got {
		cases[f.Resource.Namespace+"/"+f.Resource.Name] = f.Case
	}
	want := map[string]string{
		"shop/held-0":        podStuckTerminatingFinalizer,
		"shop/done-0":        podStuckTerminatingFinalizer,
		"logging/agent-done": podStuckTerminatingFinalizer,
		"shop/silent-0":      podStuckTerminatingNoFinalizer,
	}
	if len(got) != len(want) {
		t.Fatalf("got %d findings, want %d: %+v", len(got), len(want), got)
	}
	for k, c := range want {
		if cases[k] != c {
			t.Errorf("%s: case %q, want %q", k, cases[k], c)
		}
	}
	if other := (PodStuckTerminating{}).Evaluate(Input{Snapshot: s, Targets: []string{"worker-02"}}); len(other) != 1 || other[0].Resource.Name != "other-0" {
		t.Fatalf("worker-02: %+v", other)
	}
}
