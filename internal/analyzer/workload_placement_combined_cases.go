package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of WorkloadPlacementCombined. A finding names one; the text is below.
const (
	workloadPlacementCombinedCombined = "combined"
)

// workloadPlacementCombinedCases holds the text of each case, keyed as findings.CaseKey does.
var workloadPlacementCombinedCases = findings.Catalog{
	findings.CaseKey(WorkloadPlacementCombinedID, workloadPlacementCombinedCombined): {
		Why:         "The controller recreates the pods and they stay Pending: no node left has both the labels they require and no taint they do not tolerate. Fixing only the taints or only the labels is not enough; kubectl drain only evicts, so it completes.",
		Remediation: "Confirm with `kubectl describe node <node>` (its taints) and `kubectl get nodes --show-labels` (its labels), and for a pod of the workload `kubectl get pod -n <namespace> <pod> -o jsonpath='{.spec.nodeSelector}'`, `kubectl get pod -n <namespace> <pod> -o jsonpath='{.spec.affinity.nodeAffinity}'` and `kubectl get pod -n <namespace> <pod> -o jsonpath='{.spec.tolerations}'`. Then, safest first: pick one node to take the workload and fix both of its reasons, that is add the missing label if a label rules it out and a toleration to the pod template (or remove the taint if it is not meant) if a taint rules it out; or add a node that already has the labels and no such taint; or drain fewer nodes at once, so a node that fits stays. Changing only the cause on one side leaves the other node ruled out. The drain itself completes either way; afterwards `kubectl get pods -A --field-selector=status.phase=Pending` shows what is waiting. Only NoSchedule and NoExecute taints and required constraints are checked, not preferred ones, capacity or pod anti-affinity.",
	},
}
