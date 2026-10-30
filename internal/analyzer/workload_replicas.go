package analyzer

import "github.com/Veinar/draincheck/internal/inventory"

// workloadKey identifies a Deployment or StatefulSet, keyed like the owner of a
// pod.
type workloadKey struct{ kind, namespace, name string }

// desiredReplicas returns spec.replicas of every Deployment and StatefulSet in
// the snapshot, 1 when unset. A workload that is not in the snapshot is not in
// the map, so its replica count is unknown.
func desiredReplicas(s *inventory.Snapshot) map[workloadKey]int32 {
	desired := make(map[workloadKey]int32)
	for _, d := range s.Deployments() {
		desired[workloadKey{"Deployment", d.Namespace, d.Name}] = replicasOrDefault(d.Spec.Replicas)
	}
	for _, sts := range s.StatefulSets() {
		desired[workloadKey{"StatefulSet", sts.Namespace, sts.Name}] = replicasOrDefault(sts.Spec.Replicas)
	}
	return desired
}

// replicasOrDefault returns spec.replicas, which Kubernetes defaults to 1.
func replicasOrDefault(r *int32) int32 {
	if r == nil {
		return 1
	}
	return *r
}
