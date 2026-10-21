package analyzer

import (
	"encoding/json"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
)

func singleReplicaFinding(kind, name string, replicas int32, pods ...findings.SingleReplicaPod) findings.Finding {
	return findings.Finding{
		RuleID:   "SINGLE_REPLICA_WORKLOAD",
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Kind: kind, Namespace: "shop", Name: name},
		Evidence: findings.SingleReplicaEvidence{OwnerKind: kind, OwnerName: name, Replicas: replicas, Pods: pods},
	}
}

func TestSingleReplicaWorkload(t *testing.T) {
	s := snapshotFromFixture(t, "single-replica-workload.yaml")

	web := singleReplicaFinding("Deployment", "web", 1, findings.SingleReplicaPod{Namespace: "shop", Name: "web-7d9-aaaaa", Node: "worker-01"})
	db := singleReplicaFinding("StatefulSet", "db", 1, findings.SingleReplicaPod{Namespace: "shop", Name: "db-0", Node: "worker-01"})
	cache := singleReplicaFinding("Deployment", "cache", 1, findings.SingleReplicaPod{Namespace: "shop", Name: "cache-9f-aaaaa", Node: "worker-02"})
	surge := singleReplicaFinding("Deployment", "surge", 1,
		findings.SingleReplicaPod{Namespace: "shop", Name: "surge-2-bbbbb", Node: "worker-02"},
		findings.SingleReplicaPod{Namespace: "shop", Name: "surge-2-ccccc", Node: "worker-02"})

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "worker-01: single replicas reported; 3 replicas, finished, DaemonSet, static and unknown workloads are not",
			targets: []string{"worker-01"},
			want:    []findings.Finding{web, db},
		},
		{
			name:    "worker-02: unset replicas default to 1; a rollout's two pods are one finding",
			targets: []string{"worker-02"},
			want:    []findings.Finding{cache, surge},
		},
		{
			name:    "both nodes",
			targets: []string{"worker-01", "worker-02"},
			want:    []findings.Finding{web, db, cache, surge},
		},
		{
			name:    "a node that holds no pod gives nothing",
			targets: []string{"worker-03"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SingleReplicaWorkload{}.Evaluate(Input{Snapshot: s, Targets: tt.targets})
			findings.Sort(got)
			want := slices.Clone(tt.want)
			findings.Sort(want)
			if !reflect.DeepEqual(got, withText(want, got)) {
				t.Fatalf("findings mismatch\n got: %+v\nwant: %+v", got, want)
			}
		})
	}
}

// Each pod below sits on worker-01 and belongs to a workload that must not be
// reported: see the edge cases at the end of single-replica-workload.yaml.
func TestSingleReplicaWorkloadEdgeCases(t *testing.T) {
	s := snapshotFromFixture(t, "single-replica-workload.yaml")
	got := SingleReplicaWorkload{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}})

	var names []string
	for _, f := range got {
		names = append(names, f.Resource.Namespace+"/"+f.Resource.Name)
	}
	slices.Sort(names)
	if want := []string{"shop/db", "shop/web"}; !slices.Equal(names, want) {
		t.Errorf("reported %v, want %v", names, want)
	}
}

func TestSingleReplicaWorkloadTerminatingPod(t *testing.T) {
	s := snapshotFromFixture(t, "single-replica-workload.yaml")
	for _, f := range (SingleReplicaWorkload{}).Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}}) {
		if f.Resource.Name == "leaving" {
			t.Errorf("a pod that is already terminating was reported: %+v", f)
		}
	}
}

func TestSingleReplicaWorkloadNoWorkloadsInSnapshot(t *testing.T) {
	s := snapshotFromFixture(t, "single-replica-workload.yaml")
	bare := inventory.NewSnapshot(s.Nodes(), s.Pods(), s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp())
	if got := (SingleReplicaWorkload{}).Evaluate(Input{Snapshot: bare, Targets: []string{"worker-01", "worker-02"}}); len(got) != 0 {
		t.Errorf("a snapshot with no workloads gave %+v", got)
	}
}

// Every pod entry carries namespace, name and node, as the pods of the other
// rules do (see issue #86), and its namespace is the finding's.
func TestSingleReplicaWorkloadPodShape(t *testing.T) {
	s := snapshotFromFixture(t, "single-replica-workload.yaml")
	got := SingleReplicaWorkload{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01", "worker-02"}})
	if len(got) == 0 {
		t.Fatal("no findings")
	}
	for _, f := range got {
		raw, err := json.Marshal(f.Evidence)
		if err != nil {
			t.Fatal(err)
		}
		var ev struct {
			Pods []map[string]string `json:"pods"`
		}
		if err := json.Unmarshal(raw, &ev); err != nil {
			t.Fatal(err)
		}
		if len(ev.Pods) == 0 {
			t.Errorf("%s: no pods in evidence", f.Resource.Name)
		}
		for _, pod := range ev.Pods {
			for _, key := range []string{"namespace", "name", "node"} {
				if pod[key] == "" {
					t.Errorf("%s: pod entry %v has no %q", f.Resource.Name, pod, key)
				}
			}
			if pod["namespace"] != f.Resource.Namespace {
				t.Errorf("%s: pod namespace %q, resource namespace %q", f.Resource.Name, pod["namespace"], f.Resource.Namespace)
			}
		}
	}
}

func TestSingleReplicaWorkloadText(t *testing.T) {
	s := snapshotFromFixture(t, "single-replica-workload.yaml")
	got := SingleReplicaWorkload{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}})
	i := slices.IndexFunc(got, func(f findings.Finding) bool { return f.Resource.Name == "web" })
	if i < 0 {
		t.Fatalf("no finding for web in %+v", got)
	}
	for _, want := range []string{"Deployment shop/web", "1 replica", "worker-01", "unavailable"} {
		if !strings.Contains(got[i].Description, want) {
			t.Errorf("description %q lacks %q", got[i].Description, want)
		}
	}
	if got[i].Remediation == "" {
		t.Error("remediation is empty")
	}
}

func TestSingleReplicaWorkloadNilSnapshot(t *testing.T) {
	if got := (SingleReplicaWorkload{}).Evaluate(Input{Targets: []string{"a"}}); got != nil {
		t.Errorf("want nil, got %+v", got)
	}
}

func TestSingleReplicaWorkloadID(t *testing.T) {
	if got := (SingleReplicaWorkload{}).ID(); got != "SINGLE_REPLICA_WORKLOAD" {
		t.Errorf("ID() = %q", got)
	}
}

// The result must not depend on the order of the pods.
func TestSingleReplicaWorkloadIgnoresOrder(t *testing.T) {
	s := snapshotFromFixture(t, "single-replica-workload.yaml")
	targets := []string{"worker-01", "worker-02"}
	want := SingleReplicaWorkload{}.Evaluate(Input{Snapshot: s, Targets: targets})
	findings.Sort(want)

	pods := s.Pods()
	slices.Reverse(pods)
	reversed := inventory.NewSnapshot(s.Nodes(), pods, s.ReplicaSets(), s.PodDisruptionBudgets(), s.Timestamp(),
		inventory.WithWorkloads(inventory.Workloads{Deployments: s.Deployments(), StatefulSets: s.StatefulSets()}))
	got := SingleReplicaWorkload{}.Evaluate(Input{Snapshot: reversed, Targets: targets})
	findings.Sort(got)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order changed the result\n got: %+v\nwant: %+v", got, want)
	}
}
