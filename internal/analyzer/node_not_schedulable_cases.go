package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of NodeNotSchedulable. A finding names one; the text is below.
const (
	nodeNotSchedulableNotSchedulable = "nodes_not_schedulable"
)

// nodeNotSchedulableCases holds the text of each case, keyed as findings.CaseKey does.
var nodeNotSchedulableCases = findings.Catalog{
	findings.CaseKey(NodeNotSchedulableID, nodeNotSchedulableNotSchedulable): {
		Why:         "Pods evicted from the target nodes have fewer nodes to land on, and the count is an upper bound because taints and capacity are not checked.",
		Remediation: "Confirm with `kubectl get nodes` (SchedulingDisabled or NotReady in STATUS) and `kubectl describe node <node>` (Unschedulable, Conditions). Then, safest first: repair the not Ready nodes, or uncordon the cordoned ones once the work they wait for is done; if they must stay out, count only the other nodes as room for the evicted pods. The count in the finding is an upper bound: taints and capacity are not checked.",
	},
}
