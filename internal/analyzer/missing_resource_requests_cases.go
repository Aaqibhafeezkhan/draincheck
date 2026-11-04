package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of MissingResourceRequests. A finding names one; the text is below.
const (
	missingResourceRequestsMissingRequests = "missing_requests"
)

// missingResourceRequestsCases holds the text of each case, keyed as findings.CaseKey does.
var missingResourceRequestsCases = findings.Catalog{
	findings.CaseKey(MissingResourceRequestsID, missingResourceRequestsMissingRequests): {
		Why:         "The scheduler treats a missing request as needing nothing, so rescheduled pods may not fit where they land.",
		Remediation: "Confirm with `kubectl get pod -n <namespace> <pod> -o jsonpath='{.spec.containers[*].resources.requests}'` for a pod named in the finding (empty output: no request is set; a LimitRange or webhook default shows here). Then set CPU and memory requests on every container, or a pod-level request, in the pod template of the workload so the scheduler and capacity checks can account for it. kubectl drain evicts these pods whatever they request; the risk is where they land.",
	},
}
