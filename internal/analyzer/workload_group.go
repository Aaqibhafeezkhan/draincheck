package analyzer

import (
	"cmp"
	"slices"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
)

// workloadGroup collects the moving pods of one workload.
type workloadGroup struct {
	owner inventory.Owner
	pods  []corev1.Pod
}

// groupByWorkload groups the moving pods by their owning workload (resolved
// with inventory.OwnerResolver), sorted by namespace, kind and name.
func groupByWorkload(s *inventory.Snapshot, moving []corev1.Pod) []*workloadGroup {
	owners := inventory.NewOwnerResolver(s)
	byOwner := map[inventory.Owner]*workloadGroup{}
	var groups []*workloadGroup
	for _, pod := range moving {
		owner := owners.Resolve(pod)
		g := byOwner[owner]
		if g == nil {
			g = &workloadGroup{owner: owner}
			byOwner[owner] = g
			groups = append(groups, g)
		}
		g.pods = append(g.pods, pod)
	}
	slices.SortFunc(groups, func(a, b *workloadGroup) int {
		return cmp.Or(cmp.Compare(a.owner.Namespace, b.owner.Namespace), cmp.Compare(a.owner.Kind, b.owner.Kind), cmp.Compare(a.owner.Name, b.owner.Name))
	})
	return groups
}

// groupRoom says what the nodes left allow a workload, one cause at a time:
// taint is true when taints leave at least one node open to some pod of it,
// label when labels do. A workload with no room of either kind is ruled out by
// the causes together.
type groupRoom struct {
	taint, label bool
}

// roomOf judges the group against every node left.
func (g *workloadGroup) roomOf(nodes []corev1.Node) groupRoom {
	var room groupRoom
	for _, node := range nodes {
		p := placementOn(g.pods, node)
		room.taint = room.taint || p.TaintRoom
		room.label = room.label || p.LabelRoom
	}
	return room
}

// strandedPods lists the pods of the group, sorted by name then node.
func (g *workloadGroup) strandedPods() []findings.StrandedPod {
	out := make([]findings.StrandedPod, 0, len(g.pods))
	for _, pod := range g.pods {
		out = append(out, findings.StrandedPod{Name: pod.Name, Node: pod.Spec.NodeName})
	}
	slices.SortFunc(out, func(a, b findings.StrandedPod) int {
		return cmp.Or(cmp.Compare(a.Name, b.Name), cmp.Compare(a.Node, b.Node))
	})
	return out
}
