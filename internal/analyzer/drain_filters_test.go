package analyzer

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// filterPod is a pod as the filters see it. The cases are written from
// k8s.io/kubectl/pkg/drain filters.go (identical in v0.36.0 and v0.37.1); the
// ones marked "kind" were also observed with kubectl v1.36.1 against a
// Kubernetes v1.37.0 kind cluster, using --dry-run=server.
type filterPod struct {
	// owner is "Kind" of the controller in apps/v1 ("" for none); ownerAPI
	// overrides the apiVersion.
	owner    string
	ownerAPI string
	// notController marks the owner reference as not the controller.
	notController bool
	phase         corev1.PodPhase
	terminating   bool
	mirror        bool
	emptyDir      bool
	// nilController leaves Controller unset on the owner reference.
	nilController bool
	// before is an owner reference placed ahead of owner; volumes go before the
	// emptyDir volume.
	before  *metav1.OwnerReference
	volumes []corev1.Volume
}

func (p filterPod) pod() corev1.Pod {
	meta := metav1.ObjectMeta{Namespace: "shop", Name: "p"}
	if p.owner != "" {
		api := "apps/v1"
		if p.ownerAPI != "" {
			api = p.ownerAPI
		}
		controller := !p.notController
		ref := metav1.OwnerReference{APIVersion: api, Kind: p.owner, Name: "x", Controller: &controller}
		if p.nilController {
			ref.Controller = nil
		}
		meta.OwnerReferences = []metav1.OwnerReference{ref}
	}
	if p.before != nil {
		meta.OwnerReferences = append([]metav1.OwnerReference{*p.before}, meta.OwnerReferences...)
	}
	if p.mirror {
		meta.Annotations = map[string]string{mirrorPodAnnotation: "abc"}
	}
	if p.terminating {
		now := metav1.Now()
		meta.DeletionTimestamp = &now
	}
	pod := corev1.Pod{ObjectMeta: meta, Status: corev1.PodStatus{Phase: p.phase}}
	pod.Spec.Volumes = append(pod.Spec.Volumes, p.volumes...)
	if p.emptyDir {
		pod.Spec.Volumes = append(pod.Spec.Volumes, corev1.Volume{Name: "v", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}})
	}
	return pod
}

func TestJudgePod(t *testing.T) {
	ignoreDS := DrainOptions{IgnoreDaemonSets: true}
	force := DrainOptions{Force: true}
	emptyDir := DrainOptions{DeleteEmptyDirData: true}
	forceEmptyDir := DrainOptions{Force: true, DeleteEmptyDirData: true}
	all := DrainOptions{IgnoreDaemonSets: true, Force: true, DeleteEmptyDirData: true}

	del := drainVerdict{drainDelete, reasonNone}
	tests := []struct {
		name string
		pod  filterPod
		opts DrainOptions
		want drainVerdict
	}{
		// DaemonSet filter.
		{"DaemonSet pod is refused without --ignore-daemonsets (kind)",
			filterPod{owner: "DaemonSet", phase: corev1.PodRunning}, DrainOptions{}, drainVerdict{drainRefuse, reasonDaemonSet}},
		{"DaemonSet pod is skipped with --ignore-daemonsets (kind)",
			filterPod{owner: "DaemonSet", phase: corev1.PodRunning}, ignoreDS, drainVerdict{drainSkip, reasonDaemonSet}},
		{"--force alone does not lift the DaemonSet refusal (kind)",
			filterPod{owner: "DaemonSet", phase: corev1.PodRunning}, force, drainVerdict{drainRefuse, reasonDaemonSet}},
		{"finished DaemonSet pod needs no flag (kind, Failed)",
			filterPod{owner: "DaemonSet", phase: corev1.PodFailed}, DrainOptions{}, del},
		{"Succeeded DaemonSet pod needs no flag (source)",
			filterPod{owner: "DaemonSet", phase: corev1.PodSucceeded}, DrainOptions{}, del},
		{"terminating DaemonSet pod is still refused (no skip-wait timeout)",
			filterPod{owner: "DaemonSet", phase: corev1.PodRunning, terminating: true}, DrainOptions{}, drainVerdict{drainRefuse, reasonDaemonSet}},
		{"Pending DaemonSet pod is refused",
			filterPod{owner: "DaemonSet", phase: corev1.PodPending}, DrainOptions{}, drainVerdict{drainRefuse, reasonDaemonSet}},
		{"a DaemonSet kind from another API group is not a DaemonSet pod",
			filterPod{owner: "DaemonSet", ownerAPI: "example.com/v1", phase: corev1.PodRunning}, DrainOptions{}, del},
		{"a DaemonSet reference that is not the controller is no controller",
			filterPod{owner: "DaemonSet", notController: true, phase: corev1.PodRunning}, DrainOptions{}, drainVerdict{drainRefuse, reasonUnmanaged}},

		// Mirror (static) pod filter.
		{"static pod is skipped without any flag",
			filterPod{mirror: true, phase: corev1.PodRunning}, DrainOptions{}, drainVerdict{drainSkip, reasonMirror}},
		{"finished static pod is skipped too (source)",
			filterPod{mirror: true, phase: corev1.PodFailed}, DrainOptions{}, drainVerdict{drainSkip, reasonMirror}},
		{"static pod with a Node owner is skipped",
			filterPod{owner: "Node", ownerAPI: "v1", mirror: true, phase: corev1.PodRunning}, DrainOptions{}, drainVerdict{drainSkip, reasonMirror}},
		{"the DaemonSet filter runs before the static one (source)",
			filterPod{owner: "DaemonSet", mirror: true, phase: corev1.PodRunning}, DrainOptions{}, drainVerdict{drainRefuse, reasonDaemonSet}},
		{"static DaemonSet pod with --ignore-daemonsets stops at the DaemonSet filter",
			filterPod{owner: "DaemonSet", mirror: true, phase: corev1.PodRunning}, ignoreDS, drainVerdict{drainSkip, reasonDaemonSet}},

		// Local storage filter, then unreplicated filter.
		{"emptyDir pod with a controller is refused without --delete-emptydir-data",
			filterPod{owner: "ReplicaSet", phase: corev1.PodRunning, emptyDir: true}, DrainOptions{}, drainVerdict{drainRefuse, reasonLocalStorage}},
		{"emptyDir pod with a controller is deleted with --delete-emptydir-data",
			filterPod{owner: "ReplicaSet", phase: corev1.PodRunning, emptyDir: true}, emptyDir, del},
		{"emptyDir pod without a controller: local storage is named first (kind)",
			filterPod{phase: corev1.PodRunning, emptyDir: true}, DrainOptions{}, drainVerdict{drainRefuse, reasonLocalStorage}},
		{"--force alone does not lift the local storage refusal (kind)",
			filterPod{phase: corev1.PodRunning, emptyDir: true}, force, drainVerdict{drainRefuse, reasonLocalStorage}},
		{"--delete-emptydir-data alone moves the refusal to no controller (kind)",
			filterPod{phase: corev1.PodRunning, emptyDir: true}, emptyDir, drainVerdict{drainRefuse, reasonUnmanaged}},
		{"emptyDir pod without a controller passes with both flags (kind)",
			filterPod{phase: corev1.PodRunning, emptyDir: true}, forceEmptyDir, drainVerdict{drainDelete, reasonUnmanaged}},
		{"finished emptyDir pod needs no flag",
			filterPod{owner: "Job", phase: corev1.PodSucceeded, emptyDir: true}, DrainOptions{}, del},
		{"DaemonSet filter comes before local storage",
			filterPod{owner: "DaemonSet", phase: corev1.PodRunning, emptyDir: true}, DrainOptions{}, drainVerdict{drainRefuse, reasonDaemonSet}},
		{"DaemonSet pod with emptyDir and --ignore-daemonsets is skipped, not asked about storage",
			filterPod{owner: "DaemonSet", phase: corev1.PodRunning, emptyDir: true}, ignoreDS, drainVerdict{drainSkip, reasonDaemonSet}},
		{"static pod with emptyDir is skipped, not asked about storage",
			filterPod{mirror: true, phase: corev1.PodRunning, emptyDir: true}, DrainOptions{}, drainVerdict{drainSkip, reasonMirror}},

		// Unreplicated filter.
		{"pod without a controller is refused without --force (kind)",
			filterPod{phase: corev1.PodRunning}, DrainOptions{}, drainVerdict{drainRefuse, reasonUnmanaged}},
		{"pod without a controller is deleted with --force",
			filterPod{phase: corev1.PodRunning}, force, drainVerdict{drainDelete, reasonUnmanaged}},
		{"finished pod without a controller needs no flag",
			filterPod{phase: corev1.PodFailed}, DrainOptions{}, del},
		{"terminating pod without a controller is still refused",
			filterPod{phase: corev1.PodRunning, terminating: true}, DrainOptions{}, drainVerdict{drainRefuse, reasonUnmanaged}},
		{"pod with a controller needs no flag",
			filterPod{owner: "ReplicaSet", phase: corev1.PodRunning}, DrainOptions{}, del},
		{"pod with no phase and a controller is deleted",
			filterPod{owner: "StatefulSet"}, DrainOptions{}, del},

		// Edge cases: owner references and volumes.
		{"a controller DaemonSet reference is found behind another reference",
			filterPod{owner: "DaemonSet", phase: corev1.PodRunning,
				before: &metav1.OwnerReference{APIVersion: "v1", Kind: "ConfigMap", Name: "cfg"}},
			DrainOptions{}, drainVerdict{drainRefuse, reasonDaemonSet}},
		{"a non-controller DaemonSet reference does not hide the real controller",
			filterPod{owner: "ReplicaSet", phase: corev1.PodRunning,
				before: &metav1.OwnerReference{APIVersion: "apps/v1", Kind: "DaemonSet", Name: "ds"}},
			DrainOptions{}, del},
		{"an owner reference with no Controller field is no controller",
			filterPod{owner: "ReplicaSet", nilController: true, phase: corev1.PodRunning}, DrainOptions{}, drainVerdict{drainRefuse, reasonUnmanaged}},
		{"an owner reference with no Controller field is not a DaemonSet pod",
			filterPod{owner: "DaemonSet", nilController: true, phase: corev1.PodRunning}, DrainOptions{}, drainVerdict{drainRefuse, reasonUnmanaged}},
		{"Unknown phase is not finished: a DaemonSet pod is refused",
			filterPod{owner: "DaemonSet", phase: corev1.PodUnknown}, DrainOptions{}, drainVerdict{drainRefuse, reasonDaemonSet}},
		{"no phase is not finished: a bare pod is refused",
			filterPod{}, DrainOptions{}, drainVerdict{drainRefuse, reasonUnmanaged}},
		{"emptyDir behind other volumes is found",
			filterPod{owner: "ReplicaSet", phase: corev1.PodRunning, emptyDir: true, volumes: []corev1.Volume{
				{Name: "a", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{}}},
				{Name: "b", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{}}}}},
			DrainOptions{}, drainVerdict{drainRefuse, reasonLocalStorage}},
		{"hostPath, ConfigMap, Secret and PVC volumes are not local storage",
			filterPod{owner: "ReplicaSet", phase: corev1.PodRunning, volumes: []corev1.Volume{
				{Name: "a", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/x"}}},
				{Name: "b", VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{}}},
				{Name: "c", VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{}}},
				{Name: "d", VolumeSource: corev1.VolumeSource{PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "c"}}}}},
			DrainOptions{}, del},
		{"a memory-backed emptyDir is local storage too",
			filterPod{owner: "ReplicaSet", phase: corev1.PodRunning, volumes: []corev1.Volume{
				{Name: "m", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory}}}}},
			DrainOptions{}, drainVerdict{drainRefuse, reasonLocalStorage}},
		{"finished emptyDir pod without a controller needs no flag",
			filterPod{phase: corev1.PodFailed, emptyDir: true}, DrainOptions{}, del},
		{"terminating emptyDir pod is still asked about storage",
			filterPod{owner: "ReplicaSet", phase: corev1.PodRunning, terminating: true, emptyDir: true}, DrainOptions{}, drainVerdict{drainRefuse, reasonLocalStorage}},
		{"every flag set: an ordinary pod is deleted",
			filterPod{owner: "ReplicaSet", phase: corev1.PodRunning}, all, del},
		{"every flag set: a DaemonSet pod is still skipped",
			filterPod{owner: "DaemonSet", phase: corev1.PodRunning}, all, drainVerdict{drainSkip, reasonDaemonSet}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := judgePod(tt.pod.pod(), tt.opts); got != tt.want {
				t.Errorf("judgePod = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// The flags change the verdict, never the pod.
func TestJudgePodDoesNotMutate(t *testing.T) {
	pod := filterPod{owner: "DaemonSet", phase: corev1.PodRunning, emptyDir: true, mirror: true}.pod()
	before := pod.DeepCopy()
	judgePod(pod, DrainOptions{Force: true})
	if len(pod.OwnerReferences) != len(before.OwnerReferences) || len(pod.Spec.Volumes) != len(before.Spec.Volumes) || len(pod.Annotations) != len(before.Annotations) {
		t.Error("judgePod changed the pod")
	}
}
