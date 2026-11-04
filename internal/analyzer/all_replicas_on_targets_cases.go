package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The case of AllReplicasOnTargets. A finding names it; the text is below.
const allReplicasOnTargetsAllOnTargets = "all_on_targets"

// allReplicasOnTargetsCases holds the text of each case, keyed as
// findings.CaseKey does.
var allReplicasOnTargetsCases = findings.Catalog{
	findings.CaseKey(AllReplicasOnTargetsID, allReplicasOnTargetsAllOnTargets): {
		Why:         "kubectl drain does not wait for a replacement to be Ready before it drains the next node, so the workload can be fully unavailable until one is Ready on a node outside the batch; no budget makes it wait, and it does not warn about it.",
		Remediation: "Confirm with `kubectl get pods -n <namespace> -o wide` (the NODE column shows where each replica runs) and `kubectl get pdb -n <namespace>`. Then, safest first: drain the nodes one at a time and wait until the replacements are Ready before the next; or spread the replicas over nodes outside the batch first (topologySpreadConstraints or pod anti-affinity) and wait until they are Ready; or add a PodDisruptionBudget so kubectl drain waits instead of evicting every replica. The replacements need a node outside the batch to land on (see NO_REMAINING_NODES).",
	},
}
