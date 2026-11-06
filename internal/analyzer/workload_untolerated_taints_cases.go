package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of WorkloadUntoleratedTaints. A finding names one; the text is below.
const (
	workloadUntoleratedTaintsUntolerated = "untolerated_taints"
)

// workloadUntoleratedTaintsCases holds the text of each case, keyed as findings.CaseKey does.
var workloadUntoleratedTaintsCases = findings.Catalog{
	findings.CaseKey(WorkloadUntoleratedTaintsID, workloadUntoleratedTaintsUntolerated): {
		Why:         "The controller recreates the pods and they stay Pending until a node takes them; kubectl drain only evicts, so it completes.",
		Remediation: "Confirm with `kubectl describe node <node>` for a node named in the finding (the taints are under Taints) and `kubectl get pod -n <namespace> <pod> -o jsonpath='{.spec.tolerations}'` for a pod of the workload. Then, safest first: add or uncordon a node the workload tolerates before draining; or drain fewer nodes at once; or give the workload a toleration for the taint, only if that node is meant to run it. The drain itself completes either way; afterwards `kubectl get pods -A --field-selector=status.phase=Pending` shows what is waiting. Cordons, readiness and taints are checked, not affinity or capacity.",
	},
}
