package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of POD_OWNERSHIP. A finding names one; the text is below.
const (
	podOwnershipDaemonSet          = "daemonset"
	podOwnershipStaticPod          = "static_pod"
	podOwnershipNoController       = "no_controller"
	podOwnershipNoControllerForced = "no_controller_forced"
)

// podOwnershipCases holds the text of each case, keyed as findings.CaseKey does.
var podOwnershipCases = findings.Catalog{
	findings.CaseKey(PodOwnershipID, podOwnershipDaemonSet): {
		Why:         "kubectl drain refuses DaemonSet pods without --ignore-daemonsets and never evicts them.",
		Remediation: "Confirm with `kubectl get daemonset -n <namespace> <name>` and `kubectl get pods -A --field-selector spec.nodeName=<node> -o wide`. Then drain with --ignore-daemonsets; DaemonSet pods stay until the node itself goes down. If the DaemonSet is not found, its pods are orphans, which kubectl drain treats differently.",
	},
	findings.CaseKey(PodOwnershipID, podOwnershipStaticPod): {
		Why:         "Static pods cannot be evicted or rescheduled and stop when the node goes down.",
		Remediation: "Confirm with `kubectl get pods -A --field-selector spec.nodeName=<node>` and `kubectl get pod -n <namespace> <pod> -o jsonpath='{.metadata.annotations}'` (the annotation kubernetes.io/config.mirror marks a static pod). Then plan for the static pods to be unavailable while the node is down; kubectl drain skips them.",
	},
	findings.CaseKey(PodOwnershipID, podOwnershipNoController): {
		Why:         "kubectl drain refuses a pod with no controller without --force, and nothing recreates it.",
		Remediation: "Confirm with `kubectl get pod -n <namespace> <name> -o jsonpath='{.metadata.ownerReferences}'` (empty: no controller). Then, safest first: put the workload under a controller (Deployment, StatefulSet or Job) so it is recreated; or accept its loss and drain with --force.",
	},
	findings.CaseKey(PodOwnershipID, podOwnershipNoControllerForced): {
		Why:         "With --force kubectl drain deletes a pod with no controller, and nothing recreates it.",
		Remediation: "Confirm with `kubectl get pod -n <namespace> <name> -o jsonpath='{.metadata.ownerReferences}'` (empty: no controller). Then, safest first: put the workload under a controller (Deployment, StatefulSet or Job) so it is recreated; or accept its loss.",
	},
}
