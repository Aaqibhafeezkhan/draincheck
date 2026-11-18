package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of WorkloadUnmatchedLabels. A finding names one; the text is below.
const (
	workloadUnmatchedLabelsUnmatched = "unmatched_labels"
)

// workloadUnmatchedLabelsCases holds the text of each case, keyed as findings.CaseKey does.
var workloadUnmatchedLabelsCases = findings.Catalog{
	findings.CaseKey(WorkloadUnmatchedLabelsID, workloadUnmatchedLabelsUnmatched): {
		Why:         "The controller recreates the pods and they stay Pending until a node whose labels fit them is available; kubectl drain only evicts, so it completes.",
		Remediation: "Confirm with `kubectl get nodes --show-labels` (the labels of every node) and `kubectl get pod -n <namespace> <pod> -o jsonpath='{.spec.nodeSelector}'` and `kubectl get pod -n <namespace> <pod> -o jsonpath='{.spec.affinity.nodeAffinity}'` for a pod of the workload. Then, safest first: label a node that is meant to run the workload, or add one, before draining; or drain fewer nodes at once, so a node that fits stays; or correct the nodeSelector or node affinity in the workload's pod template if it is wrong. The drain itself completes either way; afterwards `kubectl get pods -A --field-selector=status.phase=Pending` shows what is waiting. Only required constraints are checked, not preferred ones, capacity or pod anti-affinity.",
	},
}
