package findings

// Finding is a single result of a rule evaluation.
type Finding struct {
	RuleID   string      `json:"rule_id"`
	Severity Severity    `json:"severity"`
	Resource ResourceRef `json:"resource"`
	Evidence Evidence    `json:"evidence"`
	// Case names the situation of the rule this finding belongs to. The text
	// that every finding of the case shares, why it matters and what to do, is
	// in the Catalog and is not part of the finding.
	Case string `json:"case"`
	// Description holds the facts of this finding: the numbers and the names.
	Description string `json:"description"`
}
