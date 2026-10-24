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
	"k8s.io/apimachinery/pkg/types"
)

// pdbSelection is a PodDisruptionBudget with the pods on the target nodes that
// it selects and kubectl drain would evict.
type pdbSelection struct {
	pdb policyv1.PodDisruptionBudget
	// pods are the selected pods that eviction puts through the budget, sorted
	// by namespace, name and node. Pods that bypass every budget (see
	// podDrain.bypassesBudget) are not selected, and pods the eviction API lets past
	// this one (see evictedPastBudget) are in evictable instead.
	pods []findings.PDBPod
	// evictable are the selected pods that are not Ready and that eviction lets
	// through whatever the budget says, sorted like pods. Always empty for a
	// budget whose status cannot be trusted (stale, sync failed) or whose
	// policy is unknown.
	evictable []findings.PDBPod
	// unready is the number of pods that are not Ready among pods: eviction
	// refuses them like any other pod because the budget itself is unhealthy.
	unready int
	// policyKnown is false when unhealthyPodEvictionPolicy holds a value
	// draincheck does not know. Nothing is let past such a budget, and its pods
	// cannot be judged.
	policyKnown bool
	// nodes are the nodes of pods, sorted, without duplicates.
	nodes []string
	// operators are the custom-resource controllers (see operatorOwned) of the
	// pods, sorted by API group then kind, without duplicates. Empty when the
	// pods are controlled by Deployments, StatefulSets, Jobs and the like, or by
	// nothing.
	operators []findings.ManagedBy
	// overlapped is true when another budget selects one of the same evicted
	// pods (see overlappedPDBs). The numbers of such a budget are not
	// trustworthy, so the rules that judge a budget by its numbers skip it;
	// PDBOverlappingSelectors reports it.
	overlapped bool
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

		UnhealthyPodEvictionPolicy: configuredPolicy(s.pdb),
		EvictableUnhealthyPods:     s.evictable,
	}
}

// configuredPolicy is spec.unhealthyPodEvictionPolicy as written, empty when
// unset.
func configuredPolicy(pdb policyv1.PodDisruptionBudget) string {
	if pdb.Spec.UnhealthyPodEvictionPolicy == nil {
		return ""
	}
	return string(*pdb.Spec.UnhealthyPodEvictionPolicy)
}

// selectedPDBs returns every PodDisruptionBudget that selects at least one pod
// on a target node that kubectl drain would evict, in snapshot order. Pods that
// bypass every budget and pods drain never evicts (see podDrain.consultsBudget)
// are not counted, and a budget with an invalid selector is
// skipped. A pod that is not Ready and that the eviction API lets past the
// budget (see evictedPastBudget) is kept apart in evictable, and a budget left
// with no other pod is skipped. What the budget's status says is for the caller
// to judge.
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
		if slices.Contains(in.Targets, pod.Spec.NodeName) && classifyPod(pod, owners).consultsBudget() {
			evicted = append(evicted, pod)
		}
	}

	overlapped := overlappedPDBs(in)
	var out []pdbSelection
	for _, pdb := range in.Snapshot.PodDisruptionBudgets() {
		sel, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			continue
		}

		// The numbers of a budget that is stale or failed to sync cannot tell
		// which unhealthy pods are let through, so none is.
		_, known := unhealthyPolicy(pdb)
		_, failed := pdbSyncFailure(pdb)
		judged := known && !pdbStale(pdb) && !failed

		var pods, evictable []findings.PDBPod
		var operators []findings.ManagedBy
		unready := 0
		for _, pod := range evicted {
			if pod.Namespace == pdb.Namespace && sel.Matches(labels.Set(pod.Labels)) {
				entry := findings.PDBPod{Namespace: pod.Namespace, Name: pod.Name, Node: pod.Spec.NodeName}
				if judged && evictedPastBudget(pod, pdb) {
					evictable = append(evictable, entry)
					continue
				}
				pods = append(pods, entry)
				if !podReady(pod) {
					unready++
				}
				if owner := owners.Resolve(pod); operatorOwned(owner) {
					operators = append(operators, findings.ManagedBy{APIGroup: owner.APIGroup, Kind: owner.Kind})
				}
			}
		}
		if len(pods) == 0 {
			continue // nothing the budget can refuse
		}
		slices.SortFunc(operators, func(a, b findings.ManagedBy) int {
			return cmp.Or(cmp.Compare(a.APIGroup, b.APIGroup), cmp.Compare(a.Kind, b.Kind))
		})
		operators = slices.Compact(operators)
		byName := func(a, b findings.PDBPod) int {
			return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name), cmp.Compare(a.Node, b.Node))
		}
		slices.SortFunc(pods, byName)
		slices.SortFunc(evictable, byName)
		nodes := make([]string, len(pods))
		for i, p := range pods {
			nodes[i] = p.Node
		}
		slices.Sort(nodes)
		nodes = slices.Compact(nodes)

		out = append(out, pdbSelection{pdb: pdb, pods: pods, evictable: evictable, unready: unready, policyKnown: known, nodes: nodes, operators: operators, overlapped: isOverlapped(overlapped, pdb)})
	}
	return out
}

// isOverlapped reports whether pdb is in the set returned by overlappedPDBs.
func isOverlapped(set map[types.NamespacedName]struct{}, pdb policyv1.PodDisruptionBudget) bool {
	_, ok := set[types.NamespacedName{Namespace: pdb.Namespace, Name: pdb.Name}]
	return ok
}
