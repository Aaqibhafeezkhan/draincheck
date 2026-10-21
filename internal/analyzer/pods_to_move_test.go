package analyzer

import "testing"

func TestPodsToMove(t *testing.T) {
	s := snapshotFromFixture(t, "node-not-schedulable.yaml")

	tests := []struct {
		name    string
		targets []string
		want    int
	}{
		{"pods a controller recreates are counted", []string{"worker-01"}, 2},
		{"DaemonSet, static, finished and controller-less pods are not", []string{"worker-02"}, 0},
		{"pods of several targets are summed", []string{"worker-01", "worker-03"}, 3},
		{"a target with no pods moves nothing", []string{"spare-4"}, 0},
		{"no targets move nothing", nil, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := podsToMove(s, tt.targets); got != tt.want {
				t.Errorf("podsToMove(%v) = %d, want %d", tt.targets, got, tt.want)
			}
		})
	}
}
