package findings

import "fmt"

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
