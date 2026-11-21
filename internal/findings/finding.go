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
	// Verify holds read-only kubectl commands, with this finding's names filled
	// in, that confirm the facts in Description. They are advice for the
	// operator: draincheck never runs them. Empty for a finding that has none,
	// and then not written to JSON.
	Verify []string `json:"verify,omitempty"`
}
