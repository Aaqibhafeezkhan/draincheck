package analyzer

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
)

// builtinControllerGroups are the API groups of the workload controllers that
// ship with Kubernetes: the core group (ReplicationController), apps
// (Deployment, StatefulSet, ReplicaSet, DaemonSet) and batch (Job, CronJob).
// A pod controlled by anything else is controlled by a custom resource.
var builtinControllerGroups = []string{"", "apps", "batch"}

// operatorHints holds a more specific remediation for operators draincheck
// knows, by API group. A hint is added to the generic text and never changes
// the severity; an operator that is not listed gets the generic text alone.
var operatorHints = map[string]string{
	"longhorn.io": "Longhorn can block a drain while the node holds the last healthy replica of a volume: check volume health and Longhorn's node drain policy setting before draining.",
}

// operatorOwned reports whether owner is a custom resource, that is whether its
// API group is not core, apps or batch. A pod with no controller has the zero
// Owner, whose group is the core group, so it is not.
func operatorOwned(owner inventory.Owner) bool {
	return !slices.Contains(builtinControllerGroups, owner.APIGroup)
}

// genericOperatorRemediation is the advice that holds for any operator. The
// group comes from the cluster, so a backtick in it is replaced: it must not
// open a span that reads as a command.
func genericOperatorRemediation(group string) string {
	return fmt.Sprintf(
		"Budgets managed by operator %s: confirm the numbers with `kubectl get pdb -n <namespace>` (ALLOWED DISRUPTIONS) and see who manages a budget with `kubectl get pdb -n <namespace> <name> -o yaml` (labels, ownerReferences). Then check its drain documentation and the health of what it protects before draining. Do not relax minAvailable on them; the operator may recreate the budget.",
		strings.ReplaceAll(group, "`", "'"))
}

// operatorRemediation is the remediation for the budgets of the operator in
// API group: the generic advice, then the operator's own hint if it has one.
func operatorRemediation(group string) string {
	text := genericOperatorRemediation(group)
	if hint, ok := operatorHints[group]; ok {
		text += " " + hint
	}
	return text
}

// operatorGroup identifies the budgets reported together: one operator in one
// namespace.
type operatorGroup struct {
	managedBy findings.ManagedBy
	namespace string
}

// operatorManagedFindings reports budgets that allow 0 disruptions and select
// pods controlled by a custom resource. Whether such a budget stops the drain
// depends on operator state draincheck cannot see, and the operator may remove
// it when a node is cordoned, so the answer is UNKNOWN, not BLOCKED.
//
// One finding covers every budget of one operator in one namespace: an operator
// that keeps a budget per node gives one finding, not one per node. The finding
// points at the namespace. Each budget keeps its numbers, nodes and pods in the
// typed evidence. A budget whose pods have several custom owners is placed
// under the first by API group, then kind; its evidence still lists every pod.
//
// The findings are ordered by namespace, API group and kind, so the output does
// not depend on the order of the snapshot. Every selection must have at least
// one operator.
func operatorManagedFindings(sels []pdbSelection) []findings.Finding {
	byGroup := make(map[operatorGroup][]pdbSelection)
	var groups []operatorGroup
	for _, sel := range sels {
		g := operatorGroup{managedBy: sel.operators[0], namespace: sel.pdb.Namespace}
		if _, seen := byGroup[g]; !seen {
			groups = append(groups, g)
		}
		byGroup[g] = append(byGroup[g], sel)
	}
	slices.SortFunc(groups, func(a, b operatorGroup) int {
		return cmp.Or(
			cmp.Compare(a.namespace, b.namespace),
			cmp.Compare(a.managedBy.APIGroup, b.managedBy.APIGroup),
			cmp.Compare(a.managedBy.Kind, b.managedBy.Kind))
	})

	out := make([]findings.Finding, 0, len(groups))
	for _, g := range groups {
		members := byGroup[g]
		slices.SortFunc(members, func(a, b pdbSelection) int { return cmp.Compare(a.pdb.Name, b.pdb.Name) })

		pdbs := make([]findings.PDBEvidence, len(members))
		pods := make(map[findings.PDBPod]bool)
		nodes := make(map[string]bool)
		for i, m := range members {
			pdbs[i] = m.evidence()
			for _, p := range m.pods {
				pods[p] = true
			}
			for _, n := range m.nodes {
				nodes[n] = true
			}
		}

		out = append(out, findings.Finding{
			RuleID:   PDBDisruptionBlockedID,
			Severity: findings.Unknown,
			Resource: findings.ResourceRef{Kind: "Namespace", Name: g.namespace},
			Evidence: findings.OperatorManagedPDBsEvidence{
				ManagedBy: g.managedBy,
				Namespace: g.namespace,
				PDBs:      pdbs,
			},
			Description: fmt.Sprintf(
				"Operator %s (%s) manages %s in namespace %s with 0 allowed disruptions, covering %s on %s. Whether the drain stalls depends on operator state draincheck cannot see, and the operator may remove such budgets when a node is cordoned.",
				g.managedBy.APIGroup, g.managedBy.Kind, plural(len(members), "PodDisruptionBudget"), g.namespace,
				plural(len(pods), "pod"), plural(len(nodes), "target node")),
			Remediation: operatorRemediation(g.managedBy.APIGroup),
		})
	}
	return out
}
