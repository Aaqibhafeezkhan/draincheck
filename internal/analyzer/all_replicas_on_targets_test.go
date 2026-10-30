package analyzer

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func allReplicasFinding(kind, name string, replicas int32, ready int, pods ...findings.AllReplicasPod) findings.Finding {
	var nodes []string
	for _, p := range pods {
		nodes = append(nodes, p.Node)
	}
	slices.Sort(nodes)
	return findings.Finding{
		RuleID:   "ALL_REPLICAS_ON_TARGETS",
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: kind, Namespace: "shop", Name: name},
		Evidence: findings.AllReplicasEvidence{OwnerKind: kind, OwnerName: name, Replicas: replicas, ReadyPods: ready, Nodes: slices.Compact(nodes), Pods: pods},
	}
}

func arPod(name, node string) findings.AllReplicasPod {
	return findings.AllReplicasPod{Namespace: "shop", Name: name, Node: node}
}

func reportedWorkloads(got []findings.Finding) []string {
	var names []string
	for _, f := range got {
		names = append(names, f.Resource.Name)
	}
	slices.Sort(names)
	return names
}

func TestAllReplicasOnTargets(t *testing.T) {
	s := snapshotFromFixture(t, "all-replicas-on-targets.yaml")

	web := allReplicasFinding("Deployment", "web", 3, 3, arPod("web-7d9-a", "worker-01"), arPod("web-7d9-b", "worker-01"), arPod("web-7d9-c", "worker-02"))
	api := allReplicasFinding("Deployment", "api", 2, 2, arPod("api-7d9-a", "worker-01"), arPod("api-7d9-b", "worker-01"))
	starting := allReplicasFinding("Deployment", "starting", 2, 1, arPod("starting-7d9-a", "worker-01"))
	leaving := allReplicasFinding("Deployment", "leaving", 2, 1, arPod("leaving-7d9-a", "worker-01"))
	open := allReplicasFinding("Deployment", "open", 2, 2, arPod("open-7d9-a", "worker-01"), arPod("open-7d9-b", "worker-01"))
	db := allReplicasFinding("StatefulSet", "db", 2, 2, arPod("db-0", "worker-01"), arPod("db-1", "worker-02"))
	batch := allReplicasFinding("Deployment", "batch", 2, 1, arPod("batch-7d9-a", "worker-01"))

	tests := []struct {
		name    string
		targets []string
		drain   DrainOptions
		want    []findings.Finding
	}{
		{
			name:    "worker-01 alone: workloads with every ready pod there; web and db still have a pod on worker-02",
			targets: []string{"worker-01"},
			want:    []findings.Finding{api, starting, leaving, open, batch},
		},
		{
			name:    "worker-01 and worker-02: the batch also holds every ready pod of web and db",
			targets: []string{"worker-01", "worker-02"},
			want:    []findings.Finding{web, api, starting, leaving, open, db, batch},
		},
		{
			name:    "--disable-eviction: no budget is consulted, so the guarded workload goes down too",
			targets: []string{"worker-01"},
			drain:   DrainOptions{DisableEviction: true},
			want: []findings.Finding{api, starting, leaving, open, batch,
				allReplicasFinding("Deployment", "guarded", 2, 2, arPod("guarded-7d9-a", "worker-01"), arPod("guarded-7d9-b", "worker-01"))},
		},
		{
			name:    "a node that holds no pod gives nothing",
			targets: []string{"worker-03"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AllReplicasOnTargets{}.Evaluate(Input{Snapshot: s, Targets: tt.targets, Drain: tt.drain})
			findings.Sort(got)
			want := slices.Clone(tt.want)
			findings.Sort(want)
			if !reflect.DeepEqual(got, withText(want, got)) {
				t.Fatalf("findings mismatch\n got: %+v\nwant: %+v", got, want)
			}
		})
	}
}

// Each workload below has pods on worker-01 and must not be reported; see the
// header of all-replicas-on-targets.yaml for why.
func TestAllReplicasOnTargetsNotReported(t *testing.T) {
	s := snapshotFromFixture(t, "all-replicas-on-targets.yaml")
	got := reportedWorkloads(AllReplicasOnTargets{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}}))
	for _, name := range []string{"single", "stuck", "guarded", "spread", "orphan", "web", "db"} {
		if slices.Contains(got, name) {
			t.Errorf("%s was reported, got %v", name, got)
		}
	}
}

// A workload with one desired replica belongs to SINGLE_REPLICA_WORKLOAD, so
// the two rules never report the same workload.
func TestAllReplicasOnTargetsLeavesSingleReplicaToItsRule(t *testing.T) {
	s := snapshotFromFixture(t, "all-replicas-on-targets.yaml")
	in := Input{Snapshot: s, Targets: []string{"worker-01", "worker-02", "worker-03"}}
	single := reportedWorkloads(SingleReplicaWorkload{}.Evaluate(in))
	for _, name := range reportedWorkloads(AllReplicasOnTargets{}.Evaluate(in)) {
		if slices.Contains(single, name) {
			t.Errorf("%s is reported by both rules", name)
		}
	}
	if !slices.Contains(single, "single") {
		t.Fatalf("fixture has no single-replica workload, SINGLE_REPLICA_WORKLOAD reported %v", single)
	}
}

func TestAllReplicasOnTargetsNoSnapshot(t *testing.T) {
	if got := (AllReplicasOnTargets{}).Evaluate(Input{Targets: []string{"worker-01"}}); len(got) != 0 {
		t.Errorf("no snapshot gave %+v", got)
	}
}

func TestAllReplicasOnTargetsText(t *testing.T) {
	s := snapshotFromFixture(t, "all-replicas-on-targets.yaml")
	for _, f := range (AllReplicasOnTargets{}).Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}}) {
		if f.Resource.Name != "starting" {
			continue
		}
		for _, want := range []string{"Deployment shop/starting", "1 ready pod", "2 desired", "worker-01"} {
			if !strings.Contains(f.Description, want) {
				t.Errorf("description %q lacks %q", f.Description, want)
			}
		}
		if !strings.Contains(f.Remediation, "kubectl get pods -n shop") {
			t.Errorf("remediation %q has no read-only command to confirm", f.Remediation)
		}
		return
	}
	t.Fatal("shop/starting was not reported")
}
