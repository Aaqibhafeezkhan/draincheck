package findings

// Severity is the state of a finding. There is deliberately no SAFE value.
type Severity string

const (
	// Blocked means a known constraint prevents the operation under the observed state.
	Blocked Severity = "BLOCKED"
	// Risk means a condition could cause disruption or scheduling friction.
	Risk Severity = "RISK"
	// Unknown means the outcome cannot be established reliably.
	Unknown Severity = "UNKNOWN"
)

// Valid reports whether s is one of the defined severities.
func (s Severity) Valid() bool {
	switch s {
	case Blocked, Risk, Unknown:
		return true
	}
	return false
}
