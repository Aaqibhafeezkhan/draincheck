package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of PDBDisruptionBlocked for a budget draincheck judges on its own.
// The cases for an operator-managed budget are in pdb_operator_managed_cases.go.
const (
	pdbDisruptionBlockedAtZero        = "budget_at_zero"
	pdbDisruptionBlockedUnknownPolicy = "unknown_policy"
)

// pdbDisruptionBlockedCases holds the text of each case, keyed as
// findings.CaseKey does.
var pdbDisruptionBlockedCases = findings.Catalog{
	findings.CaseKey(PDBDisruptionBlockedID, pdbDisruptionBlockedAtZero): {
		Why:         "Eviction of the pods it selects is refused, and kubectl drain retries a refused eviction until --timeout, which is infinite by default, so it waits and does not complete.",
		Remediation: "Confirm with `kubectl get pdb -n <namespace> <name>` (ALLOWED DISRUPTIONS is 0) and `kubectl describe pdb -n <namespace> <name>` (selector, current and desired). Then, safest first: bring the selected pods to Ready, by adding replicas or fixing the ones that are not, until the budget allows a disruption; or, if the budget is stricter than the workload needs, relax minAvailable/maxUnavailable. Do this before draining: the drain only waits.",
	},
	findings.CaseKey(PDBDisruptionBlockedID, pdbDisruptionBlockedUnknownPolicy): {
		Why:         "Which pods eviction refuses depends on that policy, so draincheck cannot tell whether the drain stalls.",
		Remediation: "Confirm with `kubectl get pdb -n <namespace> <name> -o yaml` and read spec.unhealthyPodEvictionPolicy. If the value is not AlwaysAllow or IfHealthyBudget, look up what it does in the Kubernetes documentation for the version of the cluster, then run draincheck again. Until then, treat the budget as one that may refuse the eviction.",
	},
}
