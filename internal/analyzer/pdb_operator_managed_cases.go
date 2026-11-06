package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// pdbOperatorManaged is the case of an operator-managed budget whose operator
// draincheck has no hint for. An operator with a hint has a case of its own,
// named by operatorCaseID.
const pdbOperatorManaged = "operator_managed"

const operatorManagedWhy = "Whether the drain stalls depends on operator state draincheck cannot see, and the operator may remove such budgets when a node is cordoned."

// operatorManagedRemediation is the advice that holds for any operator. The
// operator is named in the description of the finding.
const operatorManagedRemediation = "Confirm the numbers with `kubectl get pdb -n <namespace>` (ALLOWED DISRUPTIONS) and see who manages a budget with `kubectl get pdb -n <namespace> <name> -o yaml` (labels, ownerReferences). Then check the operator's drain documentation and the health of what it protects before draining. Do not relax minAvailable on them; the operator may recreate the budget."

// operatorCaseID returns the case for the budgets of the operator in API group:
// the case of its hint when draincheck has one, the general case otherwise.
func operatorCaseID(group string) string {
	if _, ok := operatorHints[group]; ok {
		return pdbOperatorManaged + ":" + group
	}
	return pdbOperatorManaged
}

// pdbOperatorManagedCases holds the general case and one case per operator
// hint: the general advice, then the hint of the operator.
func pdbOperatorManagedCases() findings.Catalog {
	cat := findings.Catalog{
		findings.CaseKey(PDBDisruptionBlockedID, pdbOperatorManaged): {Why: operatorManagedWhy, Remediation: operatorManagedRemediation},
	}
	for group, hint := range operatorHints {
		cat[findings.CaseKey(PDBDisruptionBlockedID, operatorCaseID(group))] = findings.Case{
			Why:         operatorManagedWhy,
			Remediation: operatorManagedRemediation + " " + hint,
		}
	}
	return cat
}
