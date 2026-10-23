package analyzer

import (
	"slices"
	"testing"
)

func TestRemainingNodes(t *testing.T) {
	s := snapshotFromFixture(t, "node-not-schedulable.yaml")
	tests := []struct {
		name    string
		targets []string
		want    []string
	}{
		{"cordoned and not Ready nodes are not room", []string{"worker-01", "worker-02", "spare-4", "spare-5"}, nil},
		{"a node with no Ready condition is room", []string{"worker-01", "worker-02", "spare-4"}, []string{"spare-5"}},
		{"targets are never room", allNodes, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			for _, n := range remainingNodes(s, tt.targets) {
				got = append(got, n.Name)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("remainingNodes = %v, want %v", got, tt.want)
			}
		})
	}
}
