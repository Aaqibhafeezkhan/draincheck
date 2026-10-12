package findings

// Finding is a single result of a rule evaluation.
type Finding struct {
	RuleID      string      `json:"rule_id"`
	Severity    Severity    `json:"severity"`
	Resource    ResourceRef `json:"resource"`
	Evidence    Evidence    `json:"evidence"`
	Description string      `json:"description"`
	Remediation string      `json:"remediation"`
}
