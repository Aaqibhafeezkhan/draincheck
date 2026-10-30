package analyzer

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
)

// AllReplicasOnTargetsID is the rule ID reported by AllReplicasOnTargets.
const AllReplicasOnTargetsID = "ALL_REPLICAS_ON_TARGETS"

// AllReplicasOnTargets reports a Deployment or StatefulSet with two or more
// desired replicas whose Ready pods all run on target nodes. Draining the
// nodes together can take the workload fully down, because kubectl drain waits
// for the pods of a node to be deleted, not for their replacements to be Ready,
// before it drains the next node; anything that slows the first node gives the
// replacements time, so it depends on timing. This is the multi-node
// case: a drain of one node leaves the replicas elsewhere. kubectl drain does
// not warn about it. It is RISK: the drain completes.
//
// Which rule wins: a workload with one desired replica is left to
// SingleReplicaWorkload, where the cause is the replica count and any drain of
// the node takes it down. Here the cause is placement together with the batch,
// so the two rules never report the same workload.
//
// Only Ready pods count, and a pod that is not Ready, finished or already
// terminating is not a survivor. A workload with no Ready pod on the targets is
// skipped: the drain does not take it down. A workload whose pods a
// PodDisruptionBudget with desiredHealthy above zero covers is skipped too,
// because kubectl drain then waits for the budget instead of taking every
// replica down; the PDB rules report such a budget. With --disable-eviction no
// budget is consulted, so none protects. A workload that is not in the
// snapshot is skipped, since its replica count is unknown. One finding per
// workload, pointing at it.
type AllReplicasOnTargets struct{}

// ID implements Rule.
func (AllReplicasOnTargets) ID() string { return AllReplicasOnTargetsID }

// Evaluate implements Rule.
func (AllReplicasOnTargets) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	desired := desiredReplicas(in.Snapshot)
	owners := inventory.NewOwnerResolver(in.Snapshot)
	ready := make(map[workloadKey]int)
	survivors := make(map[workloadKey]int)
	down := make(map[workloadKey][]findings.AllReplicasPod)
	for _, pod := range in.Snapshot.Pods() {
		owner := owners.Resolve(pod)
		key := workloadKey{owner.Kind, owner.Namespace, owner.Name}
		if owner.APIGroup != "apps" || desired[key] < 2 || !podReady(pod) {
			continue
		}
		fate := classifyPod(pod, owners, in.Drain).fate()
		if fate == fateLeaving || fate == fateFinished {
			continue // going away on its own, not carrying the load
		}
		ready[key]++
		if slices.Contains(in.Targets, pod.Spec.NodeName) && fate == fateEvictedRescheduled {
			down[key] = append(down[key], findings.AllReplicasPod{Namespace: pod.Namespace, Name: pod.Name, Node: pod.Spec.NodeName})
		} else {
			survivors[key]++
		}
	}

	guarded := podsUnderBudget(in)

	var out []findings.Finding
	for key, ps := range down {
		if survivors[key] > 0 || slices.ContainsFunc(ps, func(p findings.AllReplicasPod) bool { return guarded[p.Namespace+"/"+p.Name] }) {
			continue
		}
		slices.SortFunc(ps, func(a, b findings.AllReplicasPod) int {
			return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name), cmp.Compare(a.Node, b.Node))
		})
		var nodes []string
		for _, p := range ps {
			nodes = append(nodes, p.Node)
		}
		slices.Sort(nodes)
		nodes = slices.Compact(nodes)

		n := ready[key]
		desc := fmt.Sprintf(
			"%s %s/%s has %d desired replicas and its %s run on %s, all targets of this drain. The drain evicts every one of them and kubectl drain does not wait for a replacement to be Ready before it drains the next node, so the workload can be fully unavailable until a replacement is Ready on a node outside the batch; no budget makes kubectl drain wait, and it does not warn about it.",
			key.kind, key.namespace, key.name, desired[key], countOf(n, "ready pod"), nodeList(nodes))
		if int32(n) < desired[key] {
			desc += fmt.Sprintf(" Only Ready pods carry the load: the other %d of the desired replicas are not Ready.", int(desired[key])-n)
		}
		out = append(out, findings.Finding{
			RuleID:      AllReplicasOnTargetsID,
			Severity:    findings.Risk,
			Resource:    findings.ResourceRef{Kind: key.kind, Namespace: key.namespace, Name: key.name},
			Evidence:    findings.AllReplicasEvidence{OwnerKind: key.kind, OwnerName: key.name, Replicas: desired[key], ReadyPods: n, Nodes: nodes, Pods: ps},
			Description: desc,
			Remediation: fmt.Sprintf("Confirm with `kubectl get pods -n %s -o wide` (the NODE column shows where each replica runs) and `kubectl get pdb -n %s`. Then, safest first: drain the nodes one at a time and wait until the replacements are Ready before the next; or spread the replicas over nodes outside the batch first (topologySpreadConstraints or pod anti-affinity) and wait until they are Ready; or add a PodDisruptionBudget so kubectl drain waits instead of evicting every replica. The replacements need a node outside the batch to land on (see NO_REMAINING_NODES).", key.namespace, key.namespace),
		})
	}
	return out
}

// podsUnderBudget returns "namespace/name" of every pod on a target node that a
// PodDisruptionBudget with desiredHealthy above zero selects. kubectl drain
// waits on such a budget, so the pods are not all evicted at once.
func podsUnderBudget(in Input) map[string]bool {
	under := make(map[string]bool)
	for _, sel := range selectedPDBs(in) {
		if sel.pdb.Status.DesiredHealthy <= 0 {
			continue
		}
		for _, p := range sel.pods {
			under[p.Namespace+"/"+p.Name] = true
		}
	}
	return under
}

// countOf renders "1 ready pod", "3 ready pods".
func countOf(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// nodeList renders "node a" or "nodes a, b".
func nodeList(nodes []string) string {
	if len(nodes) == 1 {
		return "node " + nodes[0]
	}
	return "nodes " + strings.Join(nodes, ", ")
}
