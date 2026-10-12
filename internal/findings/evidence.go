package findings

// Evidence is implemented by the concrete, per-rule evidence structs.
// Rules must not use map[string]interface{} so the JSON schema stays stable.
type Evidence interface {
	isEvidence()
}
