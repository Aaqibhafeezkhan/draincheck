package analyzer

import (
	"cmp"
	"slices"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// pdbSelection is a PodDisruptionBudget with the pods on the target nodes that
// it selects and kubectl drain would evict.
type pdbSelection struct {
	pdb policyv1.PodDisruptionBudget
	// pods are sorted by namespace, name and node.
	pods []findings.PDBPod
	// nodes are the nodes of pods, sorted, without duplicates.
	nodes []string
	// operators are the custom-resource controllers (see operatorOwned) of the
	// pods, sorted by API group then kind, without duplicates. Empty when the
	// pods are controlled by Deployments, StatefulSets, Jobs and the like, or by
	// nothing.
	operators []findings.ManagedBy
}

// resource is the object a finding about the budget points at.
func (s pdbSelection) resource() findings.ResourceRef {
	return findings.ResourceRef{Kind: "PodDisruptionBudget", Namespace: s.pdb.Namespace, Name: s.pdb.Name}
}

// evidence is the budget, its status numbers as reported, and the pods and
// nodes it selects.
func (s pdbSelection) evidence() findings.PDBEvidence {
	return findings.PDBEvidence{
		Namespace:          s.pdb.Namespace,
		Name:               s.pdb.Name,
		CurrentHealthy:     s.pdb.Status.CurrentHealthy,
		DesiredHealthy:     s.pdb.Status.DesiredHealthy,
		DisruptionsAllowed: s.pdb.Status.DisruptionsAllowed,
		Nodes:              s.nodes,
		Pods:               s.pods,
	}
}

// selectedPDBs returns every PodDisruptionBudget that selects at least one pod
// on a target node that kubectl drain would evict, in snapshot order. Pods in a
// terminal phase and pods drain never evicts (see skippedByDrain) are not
// counted, and a budget with an invalid selector is skipped. What the budget's
// status says is for the caller to judge.
//
// The PDB rules build on this one selection, so a budget that one of them
// reports and another one skips cannot fall between them unreported.
func selectedPDBs(in Input) []pdbSelection {
	if in.Snapshot == nil {
		return nil
	}

	owners := inventory.NewOwnerResolver(in.Snapshot)
	var evicted []corev1.Pod // pods on the target nodes that kubectl drain would evict
	for _, pod := range in.Snapshot.Pods() {
		if slices.Contains(in.Targets, pod.Spec.NodeName) && !terminal(pod) && !skippedByDrain(pod, owners) {
			evicted = append(evicted, pod)
		}
	}

	var out []pdbSelection
	for _, pdb := range in.Snapshot.PodDisruptionBudgets() {
		sel, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			continue
		}

		var pods []findings.PDBPod
		var operators []findings.ManagedBy
		for _, pod := range evicted {
			if pod.Namespace == pdb.Namespace && sel.Matches(labels.Set(pod.Labels)) {
				pods = append(pods, findings.PDBPod{Namespace: pod.Namespace, Name: pod.Name, Node: pod.Spec.NodeName})
				if owner := owners.Resolve(pod); operatorOwned(owner) {
					operators = append(operators, findings.ManagedBy{APIGroup: owner.APIGroup, Kind: owner.Kind})
				}
			}
		}
		if len(pods) == 0 {
			continue
		}
		slices.SortFunc(operators, func(a, b findings.ManagedBy) int {
			return cmp.Or(cmp.Compare(a.APIGroup, b.APIGroup), cmp.Compare(a.Kind, b.Kind))
		})
		operators = slices.Compact(operators)
		slices.SortFunc(pods, func(a, b findings.PDBPod) int {
			return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name), cmp.Compare(a.Node, b.Node))
		})
		nodes := make([]string, len(pods))
		for i, p := range pods {
			nodes[i] = p.Node
		}
		slices.Sort(nodes)
		nodes = slices.Compact(nodes)

		out = append(out, pdbSelection{pdb: pdb, pods: pods, nodes: nodes, operators: operators})
	}
	return out
}
