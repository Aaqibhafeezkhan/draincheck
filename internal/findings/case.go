package findings

// Case is the text a rule shares between every finding of one situation: why
// that situation matters and what to do about it. A finding keeps only its own
// facts and names its case, so the text is written once and printed once.
type Case struct {
	// Why is one sentence with the consequence for the drain.
	Why string `json:"why"`
	// Remediation is the verify command and the action, with placeholders such
	// as <namespace> and <name> instead of the names of one finding.
	Remediation string `json:"remediation"`
}

// CaseKey returns the key of a case in a Catalog: the rule ID and the case ID
// joined with a slash, for example "POD_OWNERSHIP/daemonset".
func CaseKey(ruleID, caseID string) string {
	return ruleID + "/" + caseID
}
