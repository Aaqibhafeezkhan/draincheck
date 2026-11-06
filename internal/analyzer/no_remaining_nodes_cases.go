package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of NoRemainingNodes. A finding names one; the text is below.
const (
	noRemainingNodesNoRemaining = "no_remaining_nodes"
)

// noRemainingNodesCases holds the text of each case, keyed as findings.CaseKey does.
var noRemainingNodesCases = findings.Catalog{
	findings.CaseKey(NoRemainingNodesID, noRemainingNodesNoRemaining): {
		Why:         "The pods it evicts are recreated by their controllers and stay Pending until a node can take them; kubectl drain only evicts, so it completes.",
		Remediation: "Confirm with `kubectl get nodes`: no node outside the drain is Ready without SchedulingDisabled. Then, safest first: add a node, or repair or uncordon one, before draining; or drain fewer nodes at once. The drain itself completes either way; afterwards `kubectl get pods -A --field-selector=status.phase=Pending` shows what is waiting. If the description names a taint on a node, `kubectl describe node <node>` shows it under Taints; remove it, or give the workload a matching toleration, only if the node is meant to run it. Cordons, readiness and taints are checked, not affinity or capacity.",
	},
}
