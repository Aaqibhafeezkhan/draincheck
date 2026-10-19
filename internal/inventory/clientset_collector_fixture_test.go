package inventory

import (
	"context"
	"slices"
	"testing"

	"k8s.io/client-go/kubernetes/fake"
)

func TestClientsetCollectorFixtures(t *testing.T) {
	tests := []struct {
		file              string
		nodes, pods, pdbs int
		wantPodNamespaces []string
	}{
		{"multi-namespace.yaml", 2, 2, 1, []string{"billing", "shop"}},
		{"no-pdbs.yaml", 1, 1, 0, []string{"shop"}},
		{"no-pods.yaml", 1, 0, 1, nil},
	}
	for _, tt := range tests {
		t.Run(tt.file, func(t *testing.T) {
			client := fake.NewClientset(loadFixture(t, tt.file)...)

			s, err := NewClientsetCollector(client, nil).Collect(context.Background())
			if err != nil {
				t.Fatalf("Collect() error = %v", err)
			}
			if got := len(s.Nodes()); got != tt.nodes {
				t.Errorf("nodes = %d, want %d", got, tt.nodes)
			}
			if got := len(s.Pods()); got != tt.pods {
				t.Errorf("pods = %d, want %d", got, tt.pods)
			}
			if got := len(s.PodDisruptionBudgets()); got != tt.pdbs {
				t.Errorf("pdbs = %d, want %d", got, tt.pdbs)
			}

			var namespaces []string
			for _, p := range s.Pods() {
				namespaces = append(namespaces, p.Namespace)
			}
			slices.Sort(namespaces)
			if !slices.Equal(namespaces, tt.wantPodNamespaces) {
				t.Errorf("pod namespaces = %v, want %v", namespaces, tt.wantPodNamespaces)
			}
		})
	}
}
