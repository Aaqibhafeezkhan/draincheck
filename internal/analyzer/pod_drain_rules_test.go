package analyzer

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/Veinar/draincheck/internal/inventory"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ruleTreatment is the test of the table in docs/RULES.md, "Pods a drain acts
// on": one pod in each state goes through every default rule, and each rule
// either reports it or stays silent. The expectations are written from that
// table, not from what the rules print.
//
// Every state gets its own cluster so that a finding can only come from that
// pod: one target node, budgets that allow 0 disruptions, and a Deployment
// with one replica when a ReplicaSet controls the pod. A pod requests no
// resources. Each state runs twice: with one budget (the pod is blocked when a
// budget is consulted for it) and with two (the pod is overlapped; the numbers
// of the budgets are not read then, see overlappedPDBs).
type podState struct {
	name string
	pod  drainPodSpec
	// fires lists the rules that report the pod when a schedulable node is left.
	fires []string
	// moves is true when the drain evicts the pod and a controller recreates
	// it, so the node rules (NODE_NOT_SCHEDULABLE, NO_REMAINING_NODES) apply.
	moves bool
}

func podStates() []podState {
	pdbRules := []string{PDBDisruptionBlockedID, PDBOverlappingSelectorsID}
	return []podState{
		{"running pod of a Deployment",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning},
			append([]string{SingleReplicaWorkloadID, MissingResourceRequestsID}, pdbRules...), true},
		{"terminating pod of a Deployment",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning, terminating: true},
			nil, false},
		{"Pending pod of a Deployment",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodPending},
			[]string{SingleReplicaWorkloadID, MissingResourceRequestsID}, true},
		{"Succeeded pod of a Job",
			drainPodSpec{controllerKind: "Job", phase: corev1.PodSucceeded},
			nil, false},
		{"Failed pod of a Deployment",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodFailed},
			nil, false},
		{"running DaemonSet pod",
			drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodRunning},
			[]string{PodOwnershipID}, false},
		{"terminating DaemonSet pod",
			drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodRunning, terminating: true},
			[]string{PodOwnershipID}, false},
		{"finished DaemonSet pod",
			drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodFailed},
			nil, false},
		{"running static pod",
			drainPodSpec{mirror: true, nodeOwner: true, phase: corev1.PodRunning},
			[]string{PodOwnershipID}, false},
		{"terminating static pod",
			drainPodSpec{mirror: true, nodeOwner: true, phase: corev1.PodRunning, terminating: true},
			[]string{PodOwnershipID}, false},
		{"running pod with no controller",
			drainPodSpec{phase: corev1.PodRunning},
			append([]string{PodOwnershipID, MissingResourceRequestsID}, pdbRules...), false},
		{"terminating pod with no controller",
			drainPodSpec{phase: corev1.PodRunning, terminating: true},
			[]string{PodOwnershipID}, false},
		{"Pending pod with no controller",
			drainPodSpec{phase: corev1.PodPending},
			[]string{PodOwnershipID, MissingResourceRequestsID}, false},
		{"Succeeded pod with no controller",
			drainPodSpec{phase: corev1.PodSucceeded},
			nil, false},
	}
}

// stateCluster builds the cluster for one pod state. spare adds a second node
// that is cordoned; budgets is the number of budgets that select the pod.
func stateCluster(spec drainPodSpec, spare bool, budgets int) *inventory.Snapshot {
	controller := true
	pod := spec.pod()
	pod.Namespace = "shop"
	pod.Name = "app-0"
	pod.Labels = map[string]string{"app": "app"}
	pod.Spec.NodeName = "worker-01"
	pod.Spec.Containers = []corev1.Container{{Name: "app", Resources: corev1.ResourceRequirements{}}}
	if spec.phase == corev1.PodRunning {
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
	}
	if spec.controllerKind == "ReplicaSet" && !spec.notController {
		pod.OwnerReferences[0].Name = "app-7f9c"
	}

	var replicaSets []appsv1.ReplicaSet
	var deployments []appsv1.Deployment
	one := int32(1)
	if spec.controllerKind == "ReplicaSet" {
		replicaSets = []appsv1.ReplicaSet{{ObjectMeta: metav1.ObjectMeta{
			Namespace: "shop", Name: "app-7f9c",
			OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", Controller: &controller}},
		}}}
		deployments = []appsv1.Deployment{{
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "app"},
			Spec:       appsv1.DeploymentSpec{Replicas: &one},
		}}
	}
	if len(pod.OwnerReferences) > 0 && pod.OwnerReferences[0].Kind != "ReplicaSet" && pod.OwnerReferences[0].Kind != "Node" {
		pod.OwnerReferences[0].APIVersion = map[string]string{"DaemonSet": "apps/v1", "Job": "batch/v1"}[pod.OwnerReferences[0].Kind]
	}

	pdb := func(name string) policyv1.PodDisruptionBudget {
		return policyv1.PodDisruptionBudget{
			ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: name, Generation: 1},
			Spec:       policyv1.PodDisruptionBudgetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "app"}}},
			Status:     policyv1.PodDisruptionBudgetStatus{ObservedGeneration: 1, CurrentHealthy: 1, DesiredHealthy: 1},
		}
	}

	nodes := []corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "worker-01"}}}
	if spare {
		nodes = append(nodes, corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "spare"},
			Spec:       corev1.NodeSpec{Unschedulable: true},
			Status:     corev1.NodeStatus{Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}}},
		})
	}
	pdbs := []policyv1.PodDisruptionBudget{pdb("pdb-a"), pdb("pdb-b")}[:budgets]
	return inventory.NewSnapshot(nodes, []corev1.Pod{pod}, replicaSets,
		pdbs, time.Time{},
		inventory.WithWorkloads(inventory.Workloads{Deployments: deployments}))
}

func firedRules(s *inventory.Snapshot) []string {
	var ids []string
	for _, r := range DefaultRules() {
		if len(r.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}})) > 0 {
			ids = append(ids, r.ID())
		}
	}
	slices.Sort(ids)
	return ids
}

func TestRulesTreatEachPodState(t *testing.T) {
	variants := []struct {
		name    string
		budgets int
		// silent is the PDB rule that does not apply to the variant.
		silent string
	}{
		{"one budget", 1, PDBOverlappingSelectorsID},
		{"two budgets", 2, PDBDisruptionBlockedID},
	}
	for _, st := range podStates() {
		for _, v := range variants {
			t.Run(st.name+" with "+v.name+" and a cordoned spare node", func(t *testing.T) {
				want := slices.DeleteFunc(slices.Clone(st.fires), func(id string) bool { return id == v.silent })
				if st.moves {
					want = append(want, NodeNotSchedulableID, NoRemainingNodesID)
				}
				slices.Sort(want)
				if got := firedRules(stateCluster(st.pod, true, v.budgets)); !slices.Equal(got, want) {
					t.Errorf("rules that fire = %v, want %v", got, want)
				}
			})
			t.Run(st.name+" with "+v.name+" and no node left", func(t *testing.T) {
				want := slices.DeleteFunc(slices.Clone(st.fires), func(id string) bool { return id == v.silent })
				if st.moves {
					want = append(want, NoRemainingNodesID)
				}
				slices.Sort(want)
				if got := firedRules(stateCluster(st.pod, false, v.budgets)); !slices.Equal(got, want) {
					t.Errorf("rules that fire = %v, want %v", got, want)
				}
			})
		}
	}
}

// Every pod state of the table is reachable, and no state is listed twice.
func TestPodStatesAreDistinct(t *testing.T) {
	seen := map[string]bool{}
	for _, st := range podStates() {
		if seen[st.name] {
			t.Errorf("state %q listed twice", st.name)
		}
		seen[st.name] = true
	}
}

// A budget that allows one disruption is exceeded by two pods the drain
// evicts. A terminating, Pending or finished pod is not one of them.
func TestBatchExceedsBudgetCountsOnlyPodsItEvicts(t *testing.T) {
	tests := []struct {
		name   string
		extras []drainPodSpec
		want   bool
	}{
		{"two running pods exceed the budget", []drainPodSpec{{controllerKind: "StatefulSet", phase: corev1.PodRunning}}, true},
		{"a terminating pod is not counted", []drainPodSpec{{controllerKind: "StatefulSet", phase: corev1.PodRunning, terminating: true}}, false},
		{"a Pending pod is not counted", []drainPodSpec{{controllerKind: "StatefulSet", phase: corev1.PodPending}}, false},
		{"a finished pod is not counted", []drainPodSpec{{controllerKind: "StatefulSet", phase: corev1.PodSucceeded}}, false},
		{"a DaemonSet pod is not counted", []drainPodSpec{{controllerKind: "DaemonSet", phase: corev1.PodRunning}}, false},
		{"a static pod is not counted", []drainPodSpec{{mirror: true, phase: corev1.PodRunning}}, false},
		{"a pod with no controller is counted", []drainPodSpec{{phase: corev1.PodRunning}}, true},
		{"one running and one terminating beside a third running one exceed it",
			[]drainPodSpec{{controllerKind: "StatefulSet", phase: corev1.PodRunning, terminating: true}, {controllerKind: "StatefulSet", phase: corev1.PodRunning}}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specs := append([]drainPodSpec{{controllerKind: "StatefulSet", phase: corev1.PodRunning}}, tt.extras...)
			var pods []corev1.Pod
			for i, spec := range specs {
				pod := spec.pod()
				pod.Namespace, pod.Name = "shop", fmt.Sprintf("db-%d", i)
				pod.Labels = map[string]string{"app": "db"}
				pod.Spec.NodeName = "worker-01"
				if spec.phase == corev1.PodRunning {
					pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
				}
				pods = append(pods, pod)
			}
			pdb := policyv1.PodDisruptionBudget{
				ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "db", Generation: 1},
				Spec:       policyv1.PodDisruptionBudgetSpec{Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "db"}}},
				Status:     policyv1.PodDisruptionBudgetStatus{ObservedGeneration: 1, CurrentHealthy: 3, DesiredHealthy: 2, DisruptionsAllowed: 1},
			}
			s := inventory.NewSnapshot([]corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "worker-01"}}}, pods, nil,
				[]policyv1.PodDisruptionBudget{pdb}, time.Time{})
			got := len(PDBBatchExceedsBudget{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}})) > 0
			if got != tt.want {
				t.Errorf("finding = %v, want %v", got, tt.want)
			}
		})
	}
}

// A pod with requests set is not reported, so a terminating pod cannot hide
// behind or add to a workload that has none.
func TestMissingRequestsIgnoresTerminatingSiblings(t *testing.T) {
	withRequests := drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning}.pod()
	withRequests.Namespace, withRequests.Name = "shop", "app-ok"
	withRequests.Spec.NodeName = "worker-01"
	withRequests.Spec.Containers = []corev1.Container{{Name: "app", Resources: corev1.ResourceRequirements{
		Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1"), corev1.ResourceMemory: resource.MustParse("1Gi")},
	}}}
	withRequests.OwnerReferences[0].Name = "app-7f9c"
	leaving := drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning, terminating: true}.pod()
	leaving.Namespace, leaving.Name = "shop", "app-old"
	leaving.Spec.NodeName = "worker-01"
	leaving.Spec.Containers = []corev1.Container{{Name: "app"}}
	leaving.OwnerReferences[0].Name = "app-7f9c"

	controller := true
	rs := appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "app-7f9c",
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "app", Controller: &controller}}}}
	s := inventory.NewSnapshot([]corev1.Node{{ObjectMeta: metav1.ObjectMeta{Name: "worker-01"}}},
		[]corev1.Pod{withRequests, leaving}, []appsv1.ReplicaSet{rs}, nil, time.Time{})
	got := MissingResourceRequests{}.Evaluate(Input{Snapshot: s, Targets: []string{"worker-01"}})
	if len(got) != 0 {
		t.Errorf("findings = %v, want none: the only pod without requests is already terminating", got)
	}
}
