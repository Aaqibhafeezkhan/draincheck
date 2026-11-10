package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of PodStuckTerminating. A finding names one; the text is below.
const (
	podStuckTerminatingFinalizer   = "finalizer"
	podStuckTerminatingNoFinalizer = "no_finalizer"
)

// podStuckTerminatingCases holds the text of each case, keyed as findings.CaseKey does.
var podStuckTerminatingCases = findings.Catalog{
	findings.CaseKey(PodStuckTerminatingID, podStuckTerminatingFinalizer): {
		Why:         "A finalizer keeps the pod in the API after its grace period, so kubectl drain waits for it and gives up at its --timeout.",
		Remediation: "Read the finalizers with `kubectl get pod -n <namespace> <name> -o jsonpath='{.metadata.finalizers}'` and find the controller that owns each one. Fix or wait for that controller before the drain; removing a finalizer by hand skips the cleanup it exists for.",
	},
	findings.CaseKey(PodStuckTerminatingID, podStuckTerminatingNoFinalizer): {
		Why:         "The pod has no finalizer, so its kubelet has not confirmed it stopped (the node may be down or the runtime stuck), and kubectl drain waits for it and gives up at its --timeout.",
		Remediation: "Check the node with `kubectl get node <node>` and the pod with `kubectl describe pod -n <namespace> <name>`. Restore the kubelet or the container runtime on that node before the drain.",
	},
}
