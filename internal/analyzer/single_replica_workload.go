package analyzer

import (
	"cmp"
	"fmt"
	"slices"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
)

// SingleReplicaWorkloadID is the rule ID reported by SingleReplicaWorkload.
const SingleReplicaWorkloadID = "SINGLE_REPLICA_WORKLOAD"

// SingleReplicaWorkload reports a Deployment or StatefulSet that desires one
// replica and has a pod on a target node. The pod is evicted before its
// replacement is Ready, so the workload is unavailable in between, whether or
// not a PodDisruptionBudget exists. kubectl drain does not warn about it. It is
// RISK: the drain completes.
//
// The count is the desired one, spec.replicas (1 when unset), not the number of
// live pods, so a rollout that briefly runs two pods is still one finding. A
// workload with a HorizontalPodAutoscaler is not special-cased. A pod whose
// workload is not in the snapshot is skipped, since its replica count is
// unknown. Terminal pods, pods already terminating (the drain is not what
// removes them) and pods kubectl drain never evicts (DaemonSet, static) are
// skipped. One finding per workload, pointing at it.
type SingleReplicaWorkload struct{}

// ID implements Rule.
func (SingleReplicaWorkload) ID() string { return SingleReplicaWorkloadID }

type workloadKey struct{ kind, namespace, name string }

// Evaluate implements Rule.
func (SingleReplicaWorkload) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	// Desired replicas of every workload that can have one, keyed like the owner
	// of a pod. spec.replicas defaults to 1 when unset.
	desired := make(map[workloadKey]int32)
	for _, d := range in.Snapshot.Deployments() {
		desired[workloadKey{"Deployment", d.Namespace, d.Name}] = replicasOrDefault(d.Spec.Replicas)
	}
	for _, sts := range in.Snapshot.StatefulSets() {
		desired[workloadKey{"StatefulSet", sts.Namespace, sts.Name}] = replicasOrDefault(sts.Spec.Replicas)
	}

	owners := inventory.NewOwnerResolver(in.Snapshot)
	pods := make(map[workloadKey][]findings.SingleReplicaPod)
	for _, pod := range in.Snapshot.Pods() {
		if !slices.Contains(in.Targets, pod.Spec.NodeName) || terminal(pod) || pod.DeletionTimestamp != nil ||
			skippedByDrain(pod, owners) {
			continue
		}
		owner := owners.Resolve(pod)
		key := workloadKey{owner.Kind, owner.Namespace, owner.Name}
		if owner.APIGroup != "apps" || desired[key] != 1 {
			continue
		}
		pods[key] = append(pods[key], findings.SingleReplicaPod{Namespace: pod.Namespace, Name: pod.Name, Node: pod.Spec.NodeName})
	}

	var out []findings.Finding
	for key, ps := range pods {
		slices.SortFunc(ps, func(a, b findings.SingleReplicaPod) int {
			return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name), cmp.Compare(a.Node, b.Node))
		})
		out = append(out, findings.Finding{
			RuleID:   SingleReplicaWorkloadID,
			Severity: findings.Risk,
			Resource: findings.ResourceRef{Kind: key.kind, Namespace: key.namespace, Name: key.name},
			Evidence: findings.SingleReplicaEvidence{OwnerKind: key.kind, OwnerName: key.name, Replicas: 1, Pods: ps},
			Description: fmt.Sprintf(
				"%s %s/%s has 1 replica and it runs on node %s. The workload is unavailable from the eviction until the new pod is Ready.",
				key.kind, key.namespace, key.name, ps[0].Node),
			Remediation: "Run at least 2 replicas, with a PodDisruptionBudget, so a drain does not take the workload down; or accept the downtime for this one.",
		})
	}
	return out
}

// replicasOrDefault returns spec.replicas, which Kubernetes defaults to 1.
func replicasOrDefault(r *int32) int32 {
	if r == nil {
		return 1
	}
	return *r
}
