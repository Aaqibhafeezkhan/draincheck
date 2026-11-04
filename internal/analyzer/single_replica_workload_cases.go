package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of SingleReplicaWorkload. A finding names one; the text is below.
const (
	singleReplicaWorkloadSingleReplica = "single_replica"
)

// singleReplicaWorkloadCases holds the text of each case, keyed as findings.CaseKey does.
var singleReplicaWorkloadCases = findings.Catalog{
	findings.CaseKey(SingleReplicaWorkloadID, singleReplicaWorkloadSingleReplica): {
		Why:         "The workload is unavailable from the eviction until the new pod is Ready, and kubectl drain does not warn about it.",
		Remediation: "Confirm with `kubectl get <kind> -n <namespace> <name> -o jsonpath='{.spec.replicas}'` (1) and `kubectl get pdb -n <namespace>`. Then, safest first: run at least 2 replicas and wait until they are Ready, with a PodDisruptionBudget so a later drain keeps one up; or accept the downtime for this one. Replicas can also be raised by an autoscaler, which draincheck does not read.",
	},
}
