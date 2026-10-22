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

// One pod in each state: only a pod that a drain evicts and a controller
// recreates needs a new home. A pod that is already terminating has its
// replacement already.
func TestPodsToMoveByPodState(t *testing.T) {
	for _, st := range podStates() {
		t.Run(st.name, func(t *testing.T) {
			s := stateCluster(st.pod, false)
			want := 0
			if st.moves {
				want = 1
			}
			if got := podsToMove(s, []string{"worker-01"}); got != want {
				t.Errorf("podsToMove = %d, want %d", got, want)
			}
			if got := podsToMove(s, []string{"elsewhere"}); got != 0 {
				t.Errorf("podsToMove on another node = %d, want 0", got)
			}
		})
	}
}
