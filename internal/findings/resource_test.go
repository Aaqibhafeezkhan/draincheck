package findings

import "testing"

func TestResourceRefString(t *testing.T) {
	tests := []struct {
		ref  ResourceRef
		want string
	}{
		{ResourceRef{Kind: "Deployment", Namespace: "billing", Name: "payments-api"}, "Deployment/billing/payments-api"},
		{ResourceRef{Kind: "Node", Name: "worker-02"}, "Node/worker-02"},
	}
	for _, tt := range tests {
		if got := tt.ref.String(); got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}
