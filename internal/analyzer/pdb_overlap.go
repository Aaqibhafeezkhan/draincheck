package analyzer

import (
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
)

// podOverlap is a pod selected by more than one PodDisruptionBudget, with those
// budgets sorted by name.
type podOverlap struct {
	pod  corev1.Pod
	pdbs []policyv1.PodDisruptionBudget
}

// overlaps returns every pod on a target node that eviction puts through the
// budget check (see podDrain.consultsBudget) and that two or more budgets of
// its namespace select, in snapshot order. A budget with an invalid selector is
// skipped: it cannot be matched reliably.
//
// This is the one definition of an overlap. PDBOverlappingSelectors reports
// these pods, and the rules that read a budget's numbers use overlappedPDBs to
// leave out the budgets involved.
func overlaps(in Input) []podOverlap {
	if in.Snapshot == nil {
		return nil
	}

	owners := inventory.NewOwnerResolver(in.Snapshot)
	pdbs := in.Snapshot.PodDisruptionBudgets()
	var out []podOverlap
	for _, pod := range in.Snapshot.Pods() {
		if !slices.Contains(in.Targets, pod.Spec.NodeName) || !classifyPod(pod, owners).consultsBudget() {
			continue
		}
		var matched []policyv1.PodDisruptionBudget
		for _, pdb := range pdbs {
			if pdb.Namespace != pod.Namespace {
				continue
			}
			sel, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
			if err != nil || !sel.Matches(labels.Set(pod.Labels)) {
				continue
			}
			matched = append(matched, pdb)
		}
		if len(matched) < 2 {
			continue
		}
		slices.SortFunc(matched, func(a, b policyv1.PodDisruptionBudget) int { return strings.Compare(a.Name, b.Name) })
		out = append(out, podOverlap{pod: pod, pdbs: matched})
	}
	return out
}

// overlappedPDBs is the set of budgets named by overlaps.
//
// When several budgets select a pod, the disruption controller logs that it
// "Chose ... arbitrarily" one of them each time the pod changes. A budget that
// was not chosen was seen to keep an older number: 0 healthy pods while every
// pod is Ready, until a later pod event chose it (#139, measured on kind,
// Kubernetes v1.37.0, see docs/RULES.md). That only the chosen budget is
// updated is the reading of those measurements, not something checked in the
// controller source. The numbers
// of these budgets say nothing reliable about the drain, which the eviction API
// refuses for the overlap whatever they say. Only overlaps on the target
// nodes are used, so a budget left out is always named by a
// PDB_OVERLAPPING_SELECTORS finding and is never hidden.
func overlappedPDBs(in Input) map[types.NamespacedName]struct{} {
	set := map[types.NamespacedName]struct{}{}
	for _, o := range overlaps(in) {
		for _, pdb := range o.pdbs {
			set[types.NamespacedName{Namespace: pdb.Namespace, Name: pdb.Name}] = struct{}{}
		}
	}
	return set
}
