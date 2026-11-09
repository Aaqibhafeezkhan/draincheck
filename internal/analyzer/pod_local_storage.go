package analyzer

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
)

// PodLocalStorageID is the rule ID reported by PodLocalStorage.
const PodLocalStorageID = "POD_LOCAL_STORAGE"

// PodLocalStorage reports pods on target nodes that use an emptyDir volume.
// kubectl drain refuses such a pod unless --delete-emptydir-data is given, and
// with the flag it deletes the pod and the data in the volume with it. Any
// emptyDir counts, whatever its medium or size limit, as in kubectl.
//
// The severity follows kubectl drain under the flags the operator passes
// (Input.Drain):
//   - BLOCKED without --delete-emptydir-data: the drain stops before it evicts
//     anything, and waiting does not change that;
//   - RISK with it: the drain proceeds and the data in the volume is lost.
//
// The answer comes from kubectl's own filters (judgePod), which run in
// kubectl's order: a DaemonSet or static pod is decided before its volumes are
// looked at, so it is not reported here, and a pod that already ran to
// completion is removed without checks. A pod that is already terminating is
// refused by kubectl too, so it is BLOCKED; with the flag it is no RISK, as the
// drain is not what removes it, and a Pending pod has no data yet. A pod with
// no controller is judged as if --force were given, because that refusal is
// POD_OWNERSHIP's: the volume is lost once the operator adds the flag.
//
// One finding is emitted per owning workload, or per pod when nothing controls
// it. Every pod and volume stays in the typed evidence.
type PodLocalStorage struct{}

// ID implements Rule.
func (PodLocalStorage) ID() string { return PodLocalStorageID }

// localStorageGroup collects the pods that share one owner.
type localStorageGroup struct {
	resource findings.ResourceRef
	owner    inventory.Owner
	severity findings.Severity
	pods     []findings.LocalStoragePod
}

// Evaluate implements Rule.
func (PodLocalStorage) Evaluate(in Input) []findings.Finding {
	if in.Snapshot == nil {
		return nil
	}

	owners := inventory.NewOwnerResolver(in.Snapshot)
	byResource := make(map[findings.ResourceRef]*localStorageGroup)
	var groups []*localStorageGroup // first-seen order, so output does not depend on map order
	for _, pod := range in.Snapshot.Pods() {
		if !slices.Contains(in.Targets, pod.Spec.NodeName) {
			continue
		}
		severity := localStorageSeverity(pod, classifyPod(pod, owners, in.Drain), in.Drain, owners)
		if severity == "" {
			continue
		}

		owner := owners.Resolve(pod)
		ref := findings.ResourceRef{Kind: owner.Kind, Namespace: owner.Namespace, Name: owner.Name}
		if owner.Unmanaged() {
			ref = findings.ResourceRef{Kind: "Pod", Namespace: pod.Namespace, Name: pod.Name}
		}
		g := byResource[ref]
		if g == nil {
			g = &localStorageGroup{resource: ref, owner: owner, severity: severity}
			byResource[ref] = g
			groups = append(groups, g)
		}
		g.pods = append(g.pods, findings.LocalStoragePod{
			Namespace: pod.Namespace, Name: pod.Name, Node: pod.Spec.NodeName, Volumes: emptyDirNames(pod),
		})
	}

	var out []findings.Finding
	for _, g := range groups {
		slices.SortFunc(g.pods, func(a, b findings.LocalStoragePod) int {
			return cmp.Or(cmp.Compare(a.Namespace, b.Namespace), cmp.Compare(a.Name, b.Name), cmp.Compare(a.Node, b.Node))
		})
		nodes := make([]string, len(g.pods))
		for i, p := range g.pods {
			nodes[i] = p.Node
		}
		slices.Sort(nodes)
		nodes = slices.Compact(nodes)

		ev := findings.LocalStorageEvidence{
			DeleteEmptyDirData: in.Drain.DeleteEmptyDirData,
			Nodes:              nodes,
			Pods:               g.pods,
		}
		if !g.owner.Unmanaged() {
			ev.OwnerKind, ev.OwnerName = g.owner.Kind, g.owner.Name
		}
		caseID, description := describeLocalStorage(g, len(nodes))
		out = append(out, findings.Finding{
			RuleID:      PodLocalStorageID,
			Severity:    g.severity,
			Resource:    g.resource,
			Evidence:    ev,
			Case:        caseID,
			Description: description,
		})
	}
	return out
}

// localStorageSeverity is what kubectl drain's local-storage check means for
// pod, or "" when it has nothing to say about it.
func localStorageSeverity(pod corev1.Pod, d podDrain, o DrainOptions, owners *inventory.OwnerResolver) findings.Severity {
	if !d.selected || d.class == classFinished || !hasEmptyDir(pod) {
		return ""
	}
	if !o.DeleteEmptyDirData {
		if v := judgePod(pod, o, owners); v.action == drainRefuse && v.reason == reasonLocalStorage {
			return findings.Blocked
		}
		return ""
	}
	if d.terminating || d.pending {
		return ""
	}
	// kubectl's refusal of a pod with no controller is POD_OWNERSHIP's finding;
	// lifting it with --force is the operator's next step, and the volume is
	// deleted then.
	o.Force = true
	if judgePod(pod, o, owners).action == drainDelete {
		return findings.Risk
	}
	return ""
}

// emptyDirNames returns the names of the emptyDir volumes of pod, sorted.
func emptyDirNames(pod corev1.Pod) []string {
	var names []string
	for _, v := range pod.Spec.Volumes {
		if v.EmptyDir != nil {
			names = append(names, v.Name)
		}
	}
	slices.Sort(names)
	return names
}

// describeLocalStorage returns the case and the description for g, which covers
// pods on nodeCount distinct nodes. The description holds the facts; what
// kubectl does under the flags in force (a refusal for BLOCKED, a deletion for
// RISK) is the Why of the case, in podLocalStorageCases.
func describeLocalStorage(g *localStorageGroup, nodeCount int) (caseID, description string) {
	if g.owner.Unmanaged() {
		pod := g.pods[0]
		description = fmt.Sprintf("Pod %s/%s on node %s has emptyDir volumes (%s).", pod.Namespace, pod.Name, pod.Node, strings.Join(pod.Volumes, ", "))
	} else {
		description = fmt.Sprintf("%s %s/%s has %s with emptyDir volumes on %s.",
			g.resource.Kind, g.resource.Namespace, g.resource.Name, plural(len(g.pods), "pod"), plural(nodeCount, "target node"))
	}
	if g.severity == findings.Risk {
		return podLocalStorageDeleted, description
	}
	return podLocalStorageRefused, description
}
