package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// The cases of PDBUnsatisfiable. A finding names one; the text is below.
const (
	pdbUnsatisfiableNeverAllows = "never_allows"
)

// pdbUnsatisfiableCases holds the text of each case, keyed as findings.CaseKey does.
var pdbUnsatisfiableCases = findings.Catalog{
	findings.CaseKey(PDBUnsatisfiableID, pdbUnsatisfiableNeverAllows): {
		Why:         "Eviction of its pods is refused however healthy the workload is, and kubectl drain retries a refused eviction until --timeout, which is infinite by default, so it waits and does not complete.",
		Remediation: "Confirm with `kubectl get pdb -n <namespace> <name>` (ALLOWED DISRUPTIONS is 0) and `kubectl get pdb -n <namespace> <name> -o yaml` (spec.minAvailable or spec.maxUnavailable against status.expectedPods). Waiting does not help: only a change to the budget or to the replica count does. Then, safest first: scale the workload above minAvailable and wait until the new pods are Ready; or, if the budget is stricter than the workload needs, relax minAvailable/maxUnavailable for the drain and restore it afterwards. Do this before draining.",
	},
}
