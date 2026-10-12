package inventory

import (
	"errors"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func snapshotWithNodes(names ...string) *Snapshot {
	nodes := make([]corev1.Node, len(names))
	for i, n := range names {
		nodes[i] = corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: n}}
	}
	return NewSnapshot(nodes, nil, nil, nil, time.Time{})
}

func TestResolveTargetNodes(t *testing.T) {
	s := snapshotWithNodes("worker-03", "worker-01", "worker-02")

	tests := []struct {
		name      string
		requested []string
		all       bool
		want      []string
	}{
		{"named nodes are sorted", []string{"worker-02", "worker-01"}, false, []string{"worker-01", "worker-02"}},
		{"duplicates collapse", []string{"worker-01", "worker-01"}, false, []string{"worker-01"}},
		{"all nodes", nil, true, []string{"worker-01", "worker-02", "worker-03"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ResolveTargetNodes(s, tt.requested, tt.all)
			if err != nil {
				t.Fatalf("error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolveTargetNodesNotFound(t *testing.T) {
	s := snapshotWithNodes("worker-01")

	_, err := ResolveTargetNodes(s, []string{"worker-09", "worker-01", "worker-05"}, false)
	var nf *NodesNotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("error = %v, want *NodesNotFoundError", err)
	}
	if want := []string{"worker-05", "worker-09"}; !reflect.DeepEqual(nf.Names, want) {
		t.Errorf("Names = %v, want %v", nf.Names, want)
	}
	if got, want := err.Error(), "node(s) not found in cluster: worker-05, worker-09"; got != want {
		t.Errorf("message = %q, want %q", got, want)
	}
}

func TestResolveTargetNodesEmptyCluster(t *testing.T) {
	if _, err := ResolveTargetNodes(snapshotWithNodes(), nil, true); err == nil {
		t.Error("expected error when selecting all nodes of an empty cluster")
	}
}

func TestResolveTargetNodesNothingRequested(t *testing.T) {
	if _, err := ResolveTargetNodes(snapshotWithNodes("worker-01"), nil, false); err == nil {
		t.Error("expected error when no nodes are requested")
	}
}
