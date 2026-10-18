package analyzer

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// pdbEv is the evidence of one budget that allows 0 disruptions.
func pdbEv(ns, name string, healthy, desired int32, nodes []string, pods ...findings.PDBPod) findings.PDBEvidence {
	return findings.PDBEvidence{Namespace: ns, Name: name, CurrentHealthy: healthy, DesiredHealthy: desired, Nodes: nodes, Pods: pods}
}

// operatorFinding is the finding for the budgets of one operator in one
// namespace: UNKNOWN, pointing at the namespace.
func operatorFinding(group, kind, ns string, pdbs ...findings.PDBEvidence) findings.Finding {
	return findings.Finding{
		RuleID:   "PDB_DISRUPTION_BLOCKED",
		Severity: findings.Unknown,
		Resource: findings.ResourceRef{Kind: "Namespace", Name: ns},
		Evidence: findings.OperatorManagedPDBsEvidence{
			ManagedBy: findings.ManagedBy{APIGroup: group, Kind: kind},
			Namespace: ns,
			PDBs:      pdbs,
		},
	}
}

func TestPDBDisruptionBlockedOperatorManaged(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-operator-managed.yaml")
	const (
		w1 = "worker-01"
		w2 = "worker-02"
		w3 = "worker-03"
		w4 = "worker-04"
	)
	agent := func(n, node string) findings.PDBEvidence {
		return pdbEv("storage-system", "agent-"+n+"-pdb", 1, 1, []string{node}, pdbPod("storage-system", "agent-"+n, node))
	}
	nodeAgent := func(ns string, pdbs ...findings.PDBEvidence) findings.Finding {
		return operatorFinding("storage.example.com", "NodeAgent", ns, pdbs...)
	}

	tests := []struct {
		name    string
		targets []string
		want    []findings.Finding
	}{
		{
			name:    "one finding per operator and namespace; plain owners stay BLOCKED",
			targets: []string{w1, w2, w3, w4, "worker-05"},
			want: []findings.Finding{
				operatorFinding("longhorn.io", "InstanceManager", "block-storage",
					pdbEv("block-storage", "instance-manager-a", 1, 1, []string{w1}, pdbPod("block-storage", "instance-manager-a", w1))),
				nodeAgent("mixed", pdbEv("mixed", "mixed-pdb", 2, 2, []string{w1, w2},
					pdbPod("mixed", "mixed-agent-0", w2), pdbPod("mixed", "mixed-web-0", w1))),
				nodeAgent("storage-system", agent("1", w1), agent("2", w2), agent("3", w3), agent("4", w4)),
				nodeAgent("storage-system-b", pdbEv("storage-system-b", "agent-1-pdb", 1, 1, []string{w2}, pdbPod("storage-system-b", "agent-1", w2))),
				operatorFinding("other.example.org", "Gadget", "twin",
					pdbEv("twin", "twin-pdb", 2, 2, []string{w4}, pdbPod("twin", "twin-a", w4), pdbPod("twin", "twin-b", w4), pdbPod("twin", "twin-c", w4))),
				operatorFinding("other.example.org", "Gadget", "widgets",
					pdbEv("widgets", "gadget-pdb", 1, 1, []string{w2}, pdbPod("widgets", "gadget-0", w2))),
				operatorFinding("other.example.org", "Widget", "widgets",
					pdbEv("widgets", "widget-pdb", 1, 1, []string{w2}, pdbPod("widgets", "widget-0", w2))),
				pdbFinding("shop", "db-pdb", 1, 1, []string{w3}, pdbPod("shop", "db-0", w3)),
				pdbFinding("shop", "legacy-pdb", 1, 1, []string{w4}, pdbPod("shop", "legacy-0", w4)),
				pdbFinding("shop", "migrate-pdb", 1, 1, []string{w1}, pdbPod("shop", "migrate-x1", w1)),
				pdbFinding("shop", "web-pdb", 2, 2, []string{w1, w2}, pdbPod("shop", "web-7f9c-a", w1), pdbPod("shop", "web-7f9c-b", w2)),
			},
		},
		{
			name:    "only pods on the target nodes decide: an operator pod elsewhere does not make the budget operator-managed",
			targets: []string{w1},
			want: []findings.Finding{
				operatorFinding("longhorn.io", "InstanceManager", "block-storage",
					pdbEv("block-storage", "instance-manager-a", 1, 1, []string{w1}, pdbPod("block-storage", "instance-manager-a", w1))),
				nodeAgent("storage-system", agent("1", w1)),
				pdbFinding("mixed", "mixed-pdb", 2, 2, []string{w1}, pdbPod("mixed", "mixed-web-0", w1)),
				pdbFinding("shop", "migrate-pdb", 1, 1, []string{w1}, pdbPod("shop", "migrate-x1", w1)),
				pdbFinding("shop", "web-pdb", 2, 2, []string{w1}, pdbPod("shop", "web-7f9c-a", w1)),
			},
		},
		{
			name:    "an operator budget that allows a disruption gives no finding",
			targets: []string{"worker-05"},
			want:    nil,
		},
		{
			name:    "no targets",
			targets: nil,
			want:    nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Run(Input{Snapshot: s, Targets: tt.targets}, PDBDisruptionBlocked{})

			// Ignore prose; it is covered by the description tests below.
			for i := range got {
				got[i].Description, got[i].Remediation = "", ""
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("findings mismatch\n got: %+v\nwant: %+v", got, tt.want)
			}
		})
	}
}

// An operator budget whose status is stale is the business of PDB_STALE_STATUS:
// the operator-managed finding must not report it as well.
func TestPDBDisruptionBlockedOperatorManagedStaleBudget(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-operator-managed.yaml")
	in := Input{Snapshot: s, Targets: []string{"worker-03"}}

	for _, f := range (PDBDisruptionBlocked{}).Evaluate(in) {
		if strings.Contains(f.Resource.String(), "outdated") || strings.Contains(f.Description, "outdated") {
			t.Errorf("PDB_DISRUPTION_BLOCKED reports the stale budget: %+v", f)
		}
	}
	stale := (PDBStaleStatus{}).Evaluate(in)
	if len(stale) != 1 || stale[0].Resource.Name != "outdated-agent-pdb" {
		t.Errorf("PDB_STALE_STATUS = %+v, want only outdated/outdated-agent-pdb", stale)
	}
}

func TestPDBDisruptionBlockedOperatorManagedDescribesFinding(t *testing.T) {
	s := snapshotFromFixture(t, "pdb-operator-managed.yaml")
	all := []string{"worker-01", "worker-02", "worker-03", "worker-04", "worker-05"}
	find := func(t *testing.T, kind, ns string) findings.Finding {
		t.Helper()
		for _, f := range (PDBDisruptionBlocked{}).Evaluate(Input{Snapshot: s, Targets: all}) {
			if ev, ok := f.Evidence.(findings.OperatorManagedPDBsEvidence); ok && ev.ManagedBy.Kind == kind && ev.Namespace == ns {
				return f
			}
		}
		t.Fatalf("no operator-managed finding for %s in %s", kind, ns)
		return findings.Finding{}
	}

	tests := []struct {
		name            string
		kind, namespace string
		wantDescription string
		wantRemediation string
	}{
		{
			name: "several budgets: counts of budgets, pods and nodes",
			kind: "NodeAgent", namespace: "storage-system",
			wantDescription: "Operator storage.example.com (NodeAgent) manages 4 PodDisruptionBudgets in namespace storage-system with 0 allowed disruptions, covering 4 pods on 4 target nodes. Whether the drain stalls depends on operator state draincheck cannot see, and the operator may remove such budgets when a node is cordoned.",
			wantRemediation: operatorRemediation("storage.example.com"),
		},
		{
			name: "one budget on one node: singular counts",
			kind: "Widget", namespace: "widgets",
			wantDescription: "Operator other.example.org (Widget) manages 1 PodDisruptionBudget in namespace widgets with 0 allowed disruptions, covering 1 pod on 1 target node. Whether the drain stalls depends on operator state draincheck cannot see, and the operator may remove such budgets when a node is cordoned.",
			wantRemediation: operatorRemediation("other.example.org"),
		},
		{
			name: "three pods on one node are three pods and one node",
			kind: "Gadget", namespace: "twin",
			wantDescription: "Operator other.example.org (Gadget) manages 1 PodDisruptionBudget in namespace twin with 0 allowed disruptions, covering 3 pods on 1 target node. Whether the drain stalls depends on operator state draincheck cannot see, and the operator may remove such budgets when a node is cordoned.",
			wantRemediation: operatorRemediation("other.example.org"),
		},
		{
			name: "an operator with a table entry gets its more specific text",
			kind: "InstanceManager", namespace: "block-storage",
			wantDescription: "Operator longhorn.io (InstanceManager) manages 1 PodDisruptionBudget in namespace block-storage with 0 allowed disruptions, covering 1 pod on 1 target node. Whether the drain stalls depends on operator state draincheck cannot see, and the operator may remove such budgets when a node is cordoned.",
			wantRemediation: operatorRemediation("longhorn.io"),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := find(t, tt.kind, tt.namespace)

			if got.Description != tt.wantDescription {
				t.Errorf("description\n got: %s\nwant: %s", got.Description, tt.wantDescription)
			}
			if got.Remediation != tt.wantRemediation {
				t.Errorf("remediation\n got: %s\nwant: %s", got.Remediation, tt.wantRemediation)
			}
		})
	}
}

// selection builds a pdbSelection for a budget ns/name over the given pods.
func selection(ns, name string, op findings.ManagedBy, pods ...findings.PDBPod) pdbSelection {
	var nodes []string
	for _, p := range pods {
		if !slices.Contains(nodes, p.Node) {
			nodes = append(nodes, p.Node)
		}
	}
	slices.Sort(nodes)
	return pdbSelection{
		pdb:       policyv1.PodDisruptionBudget{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}},
		pods:      pods,
		nodes:     nodes,
		operators: []findings.ManagedBy{op},
	}
}

// The same input in any order gives the same findings, and a pod that two
// budgets of one operator select is counted once.
func TestOperatorManagedFindings(t *testing.T) {
	agent := findings.ManagedBy{APIGroup: "storage.example.com", Kind: "NodeAgent"}
	other := findings.ManagedBy{APIGroup: "other.example.org", Kind: "Widget"}
	a := selection("storage", "a-pdb", agent, pdbPod("storage", "p1", "n1"), pdbPod("storage", "p2", "n2"))
	b := selection("storage", "b-pdb", agent, pdbPod("storage", "p2", "n2")) // p2 again
	c := selection("storage", "c-pdb", other, pdbPod("storage", "p3", "n3"))
	d := selection("apps", "d-pdb", agent, pdbPod("apps", "p4", "n4"))

	forward := operatorManagedFindings([]pdbSelection{a, b, c, d})
	backward := operatorManagedFindings([]pdbSelection{d, c, b, a})

	if !reflect.DeepEqual(forward, backward) {
		t.Errorf("the order of the input changes the result\nforward:  %+v\nbackward: %+v", forward, backward)
	}
	var where []string
	for _, f := range forward {
		ev := f.Evidence.(findings.OperatorManagedPDBsEvidence)
		var names []string
		for _, p := range ev.PDBs {
			names = append(names, p.Name)
		}
		where = append(where, ev.Namespace+" "+ev.ManagedBy.Kind+" "+strings.Join(names, ","))
	}
	// Ordered by namespace, then API group, then kind; budgets by name.
	// other.example.org sorts before storage.example.com, whatever the kind.
	if want := []string{"apps NodeAgent d-pdb", "storage Widget c-pdb", "storage NodeAgent a-pdb,b-pdb"}; !slices.Equal(where, want) {
		t.Fatalf("findings = %q, want %q", where, want)
	}
	if want := "covering 2 pods on 2 target nodes"; !strings.Contains(forward[2].Description, want) {
		t.Errorf("p2 must be counted once, want %q in: %s", want, forward[2].Description)
	}
}

func TestOperatorRemediation(t *testing.T) {
	generic := operatorRemediation("unlisted.example.org")
	for _, want := range []string{"unlisted.example.org", "drain documentation", "health of what it protects", "Do not relax minAvailable", "recreate"} {
		if !strings.Contains(generic, want) {
			t.Errorf("generic remediation should mention %q: %s", want, generic)
		}
	}

	if generic != genericOperatorRemediation("unlisted.example.org") {
		t.Errorf("an operator not in the table must get exactly the generic text, got: %s", generic)
	}

	// A listed operator gets the generic advice first, then its own hint; the
	// table never replaces the general warning.
	known := operatorRemediation("longhorn.io")
	base := genericOperatorRemediation("longhorn.io")
	if !strings.HasPrefix(known, base+" ") || len(known) <= len(base)+1 || !strings.Contains(known, "Longhorn") {
		t.Errorf("a listed operator should get its hint after the generic text\n got: %s\nbase: %s", known, base)
	}
}

func TestOperatorOwned(t *testing.T) {
	tests := []struct {
		name  string
		owner inventory.Owner
		want  bool
	}{
		{"Deployment (apps)", inventory.Owner{APIGroup: "apps", Kind: "Deployment"}, false},
		{"StatefulSet (apps)", inventory.Owner{APIGroup: "apps", Kind: "StatefulSet"}, false},
		{"Job (batch)", inventory.Owner{APIGroup: "batch", Kind: "Job"}, false},
		{"ReplicationController (core)", inventory.Owner{Kind: "ReplicationController"}, false},
		{"no controller", inventory.Owner{}, false},
		{"custom resource", inventory.Owner{APIGroup: "storage.example.com", Kind: "NodeAgent"}, true},
		{"group that is not a valid group/version is never taken for a built-in one", inventory.Owner{APIGroup: "a/b/c", Kind: "X"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := operatorOwned(tt.owner); got != tt.want {
				t.Errorf("operatorOwned(%+v) = %v, want %v", tt.owner, got, tt.want)
			}
		})
	}
}
