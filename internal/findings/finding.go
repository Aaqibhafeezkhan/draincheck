package findings

// Finding is a single result of a rule evaluation.
type Finding struct {
	RuleID   string      `json:"rule_id"`
	Severity Severity    `json:"severity"`
	Resource ResourceRef `json:"resource"`
	Evidence Evidence    `json:"evidence"`
	// Case names the situation of the rule this finding belongs to; the text for
	// it is in the Catalog. Empty while a rule has not moved to cases.
	Case        string `json:"case,omitempty"`
	Description string `json:"description"`
	Remediation string `json:"remediation"`
}
