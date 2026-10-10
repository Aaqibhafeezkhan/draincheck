// Package findings defines the strongly typed results produced by analyzer rules.
package findings

import "fmt"

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

// ResourceRef identifies the Kubernetes object a finding is about.
// Namespace is empty for cluster-scoped objects.
type ResourceRef struct {
	Kind      string
	Namespace string
	Name      string
}

// String renders "Kind/namespace/name", or "Kind/name" when cluster-scoped.
func (r ResourceRef) String() string {
	if r.Namespace == "" {
		return fmt.Sprintf("%s/%s", r.Kind, r.Name)
	}
	return fmt.Sprintf("%s/%s/%s", r.Kind, r.Namespace, r.Name)
}

// MarshalText makes ResourceRef serialize as a plain string in JSON.
func (r ResourceRef) MarshalText() ([]byte, error) {
	return []byte(r.String()), nil
}

// Evidence is implemented by the concrete, per-rule evidence structs.
// Rules must not use map[string]interface{} so the JSON schema stays stable.
type Evidence interface {
	isEvidence()
}

// PDBEvidence is the evidence for PodDisruptionBudget findings.
type PDBEvidence struct {
	Namespace          string `json:"namespace"`
	Name               string `json:"name"`
	CurrentHealthy     int32  `json:"current_healthy"`
	DesiredHealthy     int32  `json:"desired_healthy"`
	DisruptionsAllowed int32  `json:"disruptions_allowed"`
}

func (PDBEvidence) isEvidence() {}

// Finding is a single result of a rule evaluation.
type Finding struct {
	RuleID      string      `json:"rule_id"`
	Severity    Severity    `json:"severity"`
	Resource    ResourceRef `json:"resource"`
	Evidence    Evidence    `json:"evidence"`
	Description string      `json:"description"`
	Remediation string      `json:"remediation"`
}
