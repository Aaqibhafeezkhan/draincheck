package inventory

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
)

// Snapshot is an immutable, point-in-time view of the cluster objects the
// analyzer needs. Input is deep-copied on construction and every accessor
// returns a deep copy, so no caller can alter what other rules observe.
type Snapshot struct {
	nodes     []corev1.Node
	pods      []corev1.Pod
	pdbs      []policyv1.PodDisruptionBudget
	timestamp time.Time
}

// NewSnapshot builds a Snapshot from the given objects, taken at timestamp.
func NewSnapshot(nodes []corev1.Node, pods []corev1.Pod, pdbs []policyv1.PodDisruptionBudget, timestamp time.Time) *Snapshot {
	return &Snapshot{
		nodes:     copyNodes(nodes),
		pods:      copyPods(pods),
		pdbs:      copyPDBs(pdbs),
		timestamp: timestamp,
	}
}

// Nodes returns a copy of the nodes in the snapshot.
func (s *Snapshot) Nodes() []corev1.Node { return copyNodes(s.nodes) }

// Pods returns a copy of the pods in the snapshot.
func (s *Snapshot) Pods() []corev1.Pod { return copyPods(s.pods) }

// PodDisruptionBudgets returns a copy of the PodDisruptionBudgets in the snapshot.
func (s *Snapshot) PodDisruptionBudgets() []policyv1.PodDisruptionBudget { return copyPDBs(s.pdbs) }

// Timestamp is when the snapshot was taken.
func (s *Snapshot) Timestamp() time.Time { return s.timestamp }

func copyNodes(in []corev1.Node) []corev1.Node {
	out := make([]corev1.Node, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}

func copyPods(in []corev1.Pod) []corev1.Pod {
	out := make([]corev1.Pod, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}

func copyPDBs(in []policyv1.PodDisruptionBudget) []policyv1.PodDisruptionBudget {
	out := make([]policyv1.PodDisruptionBudget, len(in))
	for i := range in {
		in[i].DeepCopyInto(&out[i])
	}
	return out
}
