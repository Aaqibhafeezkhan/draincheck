package analyzer

import (
	"bytes"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

// TestVerifyCommands pins the commands of every kind of finding: the names come
// from the finding, nothing is left as a placeholder, and a finding that lists
// pods or nodes gets the first of them, sorted as the evidence sorts them.
func TestVerifyCommands(t *testing.T) {
	pdb := findings.PDBEvidence{Namespace: "shop", Name: "web-pdb"}
	pdbRes := findings.ResourceRef{Kind: "PodDisruptionBudget", Namespace: "shop", Name: "web-pdb"}
	workload := findings.ResourceRef{Kind: "Deployment", Namespace: "shop", Name: "web"}
	stranded := []findings.StrandedPod{{Name: "web-1", Node: "worker-01"}, {Name: "web-2", Node: "worker-02"}}

	tests := []struct {
		name string
		f    findings.Finding
		want []string
	}{
		{"pdb blocked", findings.Finding{Resource: pdbRes, Evidence: pdb}, []string{
			"kubectl get pdb -n shop web-pdb",
			"kubectl get pdb -n shop web-pdb -o yaml",
		}},
		{"pdb unsatisfiable", findings.Finding{Resource: pdbRes, Evidence: findings.PDBUnsatisfiableEvidence{Namespace: "shop", Name: "web-pdb"}}, []string{
			"kubectl get pdb -n shop web-pdb",
			"kubectl get pdb -n shop web-pdb -o yaml",
		}},
		{"pdb batch", findings.Finding{Resource: pdbRes, Evidence: findings.PDBBatchEvidence{PDBEvidence: pdb}}, []string{
			"kubectl get pdb -n shop web-pdb",
		}},
		{"pdb stale", findings.Finding{Resource: pdbRes, Evidence: findings.PDBStaleStatusEvidence{PDBEvidence: pdb}}, []string{
			"kubectl get pdb -n shop web-pdb -o jsonpath='{.metadata.generation} {.status.observedGeneration}'",
		}},
		{"pdb sync failed", findings.Finding{Resource: pdbRes, Evidence: findings.PDBSyncFailedEvidence{PDBEvidence: pdb}}, []string{
			"kubectl get pdb -n shop web-pdb -o jsonpath='{.status.conditions}'",
		}},
		{"pdb operator managed", findings.Finding{
			Resource: findings.ResourceRef{Kind: "Namespace", Name: "db"},
			Evidence: findings.OperatorManagedPDBsEvidence{Namespace: "db", PDBs: []findings.PDBEvidence{{Namespace: "db", Name: "pg-a"}, {Namespace: "db", Name: "pg-b"}}},
		}, []string{
			"kubectl get pdb -n db",
			"kubectl get pdb -n db pg-a -o yaml",
		}},
		{"pdb overlapping", findings.Finding{
			Resource: findings.ResourceRef{Kind: "Pod", Namespace: "shop", Name: "web-1"},
			Evidence: findings.OverlappingPDBsEvidence{Namespace: "shop", PDBs: []string{"a", "b"}},
		}, []string{
			"kubectl get pod -n shop web-1 --show-labels",
			"kubectl get pdb -n shop a -o yaml",
			"kubectl get pdb -n shop b -o yaml",
		}},
		{"node not schedulable", findings.Finding{
			Resource: findings.ResourceRef{Kind: "Node", Name: "worker-01,worker-02"},
			Evidence: findings.NodeNotSchedulableEvidence{Nodes: []findings.UnschedulableNode{{Name: "spare-1"}, {Name: "spare-2"}}},
		}, []string{
			"kubectl get node spare-1 spare-2",
			"kubectl describe node spare-1",
		}},
		{"no remaining nodes", findings.Finding{Resource: findings.ResourceRef{Kind: "Node", Name: "a,b"}, Evidence: findings.NoRemainingNodesEvidence{}}, []string{
			"kubectl get nodes",
			"kubectl get pods -A --field-selector=status.phase=Pending",
		}},
		{"untolerated taints", findings.Finding{Resource: workload, Evidence: findings.WorkloadUntoleratedTaintsEvidence{
			Pods: stranded, TaintedNodes: []findings.TaintedNode{{Name: "gpu-1"}, {Name: "gpu-2"}},
		}}, []string{
			"kubectl describe node gpu-1",
			"kubectl get pod -n shop web-1 -o jsonpath='{.spec.tolerations}'",
		}},
		{"unmatched labels", findings.Finding{Resource: workload, Evidence: findings.WorkloadUnmatchedLabelsEvidence{Pods: stranded}}, []string{
			"kubectl get nodes --show-labels",
			"kubectl get pod -n shop web-1 -o jsonpath='{.spec.nodeSelector}'",
			"kubectl get pod -n shop web-1 -o jsonpath='{.spec.affinity.nodeAffinity}'",
		}},
		{"placement combined", findings.Finding{Resource: workload, Evidence: findings.WorkloadPlacementCombinedEvidence{
			Pods: stranded, RuledOutNodes: []findings.RuledOutNode{{Name: "spare-1"}},
		}}, []string{
			"kubectl describe node spare-1",
			"kubectl get nodes --show-labels",
			"kubectl get pod -n shop web-1 -o jsonpath='{.spec.tolerations}'",
		}},
		{"single replica", findings.Finding{Resource: workload, Evidence: findings.SingleReplicaEvidence{}}, []string{
			"kubectl get Deployment -n shop web -o jsonpath='{.spec.replicas}'",
			"kubectl get pdb -n shop",
		}},
		{"all replicas", findings.Finding{Resource: workload, Evidence: findings.AllReplicasEvidence{}}, []string{
			"kubectl get pods -n shop -o wide",
			"kubectl get pdb -n shop",
		}},
		{"ownership daemonset", findings.Finding{
			Resource: findings.ResourceRef{Kind: "DaemonSet", Namespace: "kube-system", Name: "agent"},
			Evidence: findings.OwnershipEvidence{Category: findings.OwnershipDaemonSet},
		}, []string{"kubectl get daemonset -n kube-system agent"}},
		{"ownership orphaned daemonset", findings.Finding{
			Resource: findings.ResourceRef{Kind: "DaemonSet", Namespace: "kube-system", Name: "agent"},
			Evidence: findings.OwnershipEvidence{Category: findings.OwnershipOrphanedDaemonSet},
		}, []string{"kubectl get daemonset -n kube-system agent"}},
		{"ownership static", findings.Finding{
			Resource: findings.ResourceRef{Kind: "Node", Name: "worker-01"},
			Evidence: findings.OwnershipEvidence{Category: findings.OwnershipStaticPod, Nodes: []string{"worker-01"},
				Pods: []findings.OwnershipPod{{Namespace: "kube-system", Name: "etcd-worker-01", Node: "worker-01"}}},
		}, []string{
			"kubectl get pods -A --field-selector spec.nodeName=worker-01",
			"kubectl get pod -n kube-system etcd-worker-01 -o jsonpath='{.metadata.annotations}'",
		}},
		{"ownership unmanaged", findings.Finding{
			Resource: findings.ResourceRef{Kind: "Pod", Namespace: "shop", Name: "tool"},
			Evidence: findings.OwnershipEvidence{Category: findings.OwnershipUnmanaged},
		}, []string{"kubectl get pod -n shop tool -o jsonpath='{.metadata.ownerReferences}'"}},
		{"local storage", findings.Finding{Resource: workload, Evidence: findings.LocalStorageEvidence{
			Pods: []findings.LocalStoragePod{{Namespace: "shop", Name: "web-1"}},
		}}, []string{"kubectl get pod -n shop web-1 -o jsonpath='{.spec.volumes}'"}},
		{"missing requests", findings.Finding{Resource: workload, Evidence: findings.MissingRequestsEvidence{
			Pods: []findings.MissingRequestsPod{{Name: "web-1"}},
		}}, []string{"kubectl get pod -n shop web-1 -o jsonpath='{.spec.containers[*].resources.requests}'"}},
		{"stuck terminating", findings.Finding{
			Resource: findings.ResourceRef{Kind: "Pod", Namespace: "shop", Name: "web-1"},
			Evidence: findings.StuckTerminatingEvidence{Node: "worker-01"},
		}, []string{
			"kubectl get pod -n shop web-1 -o jsonpath='{.metadata.finalizers}'",
			"kubectl get node worker-01",
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := verifyCommands(tc.f); !slices.Equal(got, tc.want) {
				t.Errorf("commands differ\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// A finding with no evidence of a known kind, or one whose list of pods or
// nodes is empty, gets no command that needs the missing name, rather than one
// with a hole in it.
func TestVerifyCommandsNeverLeavesAHole(t *testing.T) {
	workload := findings.ResourceRef{Kind: "Deployment", Namespace: "shop", Name: "web"}
	tests := []struct {
		name string
		f    findings.Finding
		want []string
	}{
		{"no evidence", findings.Finding{Resource: workload}, nil},
		{"unmatched labels without pods", findings.Finding{Resource: workload, Evidence: findings.WorkloadUnmatchedLabelsEvidence{}}, []string{"kubectl get nodes --show-labels"}},
		{"local storage without pods", findings.Finding{Resource: workload, Evidence: findings.LocalStorageEvidence{}}, nil},
		{"missing requests without pods", findings.Finding{Resource: workload, Evidence: findings.MissingRequestsEvidence{}}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := verifyCommands(tc.f); !slices.Equal(got, tc.want) {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// AddVerify fills Verify on a copy, so a rule does not set it and a new rule
// cannot forget it (TestAdviceContract checks every default rule).
func TestAddVerifyFillsAndLeavesInputAlone(t *testing.T) {
	in := Run(nil, Input{Snapshot: snapshotFromFixture(t, "single-replica-workload.yaml"), Targets: []string{"worker-01", "worker-02"}}, SingleReplicaWorkload{})
	if len(in) == 0 {
		t.Fatal("the fixture gives no finding")
	}

	got := AddVerify(nil, in)

	if len(got) != len(in) {
		t.Fatalf("got %d findings, want %d", len(got), len(in))
	}
	for i, f := range got {
		if len(f.Verify) == 0 {
			t.Errorf("%s: Verify is empty", f.Resource)
		}
		if len(in[i].Verify) != 0 {
			t.Errorf("%s: the input finding was changed", in[i].Resource)
		}
	}
}

// The commands are advice added after the rules ran, so the log says how many
// findings got some and how many commands that is.
func TestAddVerifyLogsWhatItAdded(t *testing.T) {
	pod := findings.Finding{
		RuleID:   "A",
		Resource: findings.ResourceRef{Kind: "Pod", Namespace: "web", Name: "x"},
		Evidence: findings.StuckTerminatingEvidence{Node: "worker-01"},
	}
	bare := findings.Finding{RuleID: "A", Resource: findings.ResourceRef{Kind: "Pod", Namespace: "web", Name: "y"}}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	AddVerify(logger, []findings.Finding{pod, bare})

	if want := `msg="verify commands" findings=1 commands=2`; !strings.Contains(buf.String(), want) {
		t.Errorf("log lacks %q:\n%s", want, buf.String())
	}
}

func TestAddVerifyNoFindings(t *testing.T) {
	if got := AddVerify(nil, nil); len(got) != 0 {
		t.Errorf("got %+v, want none", got)
	}
}
