package analyzer

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func emptyDirVolume(name string) corev1.Volume {
	return corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}
}

func memoryEmptyDir(name string) corev1.Volume {
	limit := resource.MustParse("64Mi")
	return corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{Medium: corev1.StorageMediumMemory, SizeLimit: &limit},
	}}
}

func configMapVolume(name string) corev1.Volume {
	return corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{
		ConfigMap: &corev1.ConfigMapVolumeSource{LocalObjectReference: corev1.LocalObjectReference{Name: name}},
	}}
}

func hostPathVolume(name string) corev1.Volume {
	return corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/var/" + name}}}
}

func claimVolume(name string) corev1.Volume {
	return corev1.Volume{Name: name, VolumeSource: corev1.VolumeSource{
		PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: name},
	}}
}

// storagePod is a pod on worker-01 with the given volumes. A Deployment pod
// belongs to the ReplicaSet web-7f9c, which the test cluster resolves to the
// Deployment web.
func storagePod(name string, spec drainPodSpec, volumes ...corev1.Volume) corev1.Pod {
	pod := spec.pod()
	pod.Name = name
	pod.Spec.NodeName = "worker-01"
	pod.Spec.Volumes = volumes
	if len(pod.OwnerReferences) > 0 && pod.OwnerReferences[0].Kind == "DaemonSet" {
		pod.OwnerReferences[0].APIVersion = "apps/v1"
	}
	return pod
}

func foreignDaemonSetPod() corev1.Pod {
	pod := storagePod("p", drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodRunning}, emptyDirVolume("cache"))
	pod.OwnerReferences[0].APIVersion = "example.com/v1"
	return pod
}

var running = drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning}

func storageSnapshot(pods ...corev1.Pod) *inventory.Snapshot {
	controller := true
	rss := []appsv1.ReplicaSet{{ObjectMeta: metav1.ObjectMeta{
		Namespace: "shop", Name: "web-7f9c",
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "web", Controller: &controller}},
	}}}
	return inventory.NewSnapshot(nil, pods, rss, nil, time.Time{})
}

func storageFindings(pods []corev1.Pod, drain DrainOptions, targets ...string) []findings.Finding {
	if targets == nil {
		targets = []string{"worker-01"}
	}
	return PodLocalStorage{}.Evaluate(Input{Snapshot: storageSnapshot(pods...), Targets: targets, Drain: drain})
}

// The rule asks kubectl's own filters: a refusal is BLOCKED, and with
// --delete-emptydir-data the data is deleted, which is a RISK.
func TestPodLocalStorageSeverity(t *testing.T) {
	tests := []struct {
		name  string
		pod   corev1.Pod
		drain DrainOptions
		want  findings.Severity // "" for no finding
	}{
		{"emptyDir without the flag is refused", storagePod("p", running, emptyDirVolume("cache")), DrainOptions{}, findings.Blocked},
		{"emptyDir with the flag is deleted", storagePod("p", running, emptyDirVolume("cache")), DrainOptions{DeleteEmptyDirData: true}, findings.Risk},
		{"medium Memory counts as emptyDir, as in kubectl", storagePod("p", running, memoryEmptyDir("tmp")), DrainOptions{}, findings.Blocked},
		{"medium Memory with the flag", storagePod("p", running, memoryEmptyDir("tmp")), DrainOptions{DeleteEmptyDirData: true}, findings.Risk},
		{"no volumes", storagePod("p", running), DrainOptions{}, ""},
		{"configMap and hostPath and claim are not local storage for kubectl",
			storagePod("p", running, configMapVolume("cfg"), hostPathVolume("logs"), claimVolume("data")), DrainOptions{}, ""},
		{"the flag changes nothing for a pod without emptyDir", storagePod("p", running, claimVolume("data")), DrainOptions{DeleteEmptyDirData: true}, ""},

		{"--force and --ignore-daemonsets and --disable-eviction do not matter",
			storagePod("p", running, emptyDirVolume("cache")), DrainOptions{Force: true, IgnoreDaemonSets: true, DisableEviction: true}, findings.Blocked},

		{"Succeeded pod is removed without checks",
			storagePod("p", drainPodSpec{controllerKind: "Job", phase: corev1.PodSucceeded}, emptyDirVolume("cache")), DrainOptions{}, ""},
		{"Failed pod is removed without checks",
			storagePod("p", drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodFailed}, emptyDirVolume("cache")), DrainOptions{}, ""},
		{"Failed pod with the flag",
			storagePod("p", drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodFailed}, emptyDirVolume("cache")), DrainOptions{DeleteEmptyDirData: true}, ""},

		// kubectl checks the DaemonSet and static filters before local storage.
		{"DaemonSet pod without --ignore-daemonsets: kubectl names the DaemonSet, not the volume",
			storagePod("p", drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodRunning}, emptyDirVolume("cache")), DrainOptions{}, ""},
		{"DaemonSet pod with --ignore-daemonsets is skipped",
			storagePod("p", drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodRunning}, emptyDirVolume("cache")), DrainOptions{IgnoreDaemonSets: true}, ""},
		{"static pod is skipped",
			storagePod("p", drainPodSpec{mirror: true, nodeOwner: true, phase: corev1.PodRunning}, emptyDirVolume("cache")), DrainOptions{}, ""},
		{"static pod with a DaemonSet owner is skipped",
			storagePod("p", drainPodSpec{mirror: true, controllerKind: "DaemonSet", phase: corev1.PodRunning}, emptyDirVolume("cache")), DrainOptions{}, ""},

		// A terminating pod is still checked by kubectl (a refusal), but the flag
		// makes it no risk: the pod is already leaving.
		{"terminating pod is still refused",
			storagePod("p", drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning, terminating: true}, emptyDirVolume("cache")), DrainOptions{}, findings.Blocked},
		{"terminating pod with the flag loses nothing the drain causes",
			storagePod("p", drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning, terminating: true}, emptyDirVolume("cache")), DrainOptions{DeleteEmptyDirData: true}, ""},
		{"Pending pod is refused",
			storagePod("p", drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodPending}, emptyDirVolume("cache")), DrainOptions{}, findings.Blocked},
		{"Pending pod with the flag has no data to lose",
			storagePod("p", drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodPending}, emptyDirVolume("cache")), DrainOptions{DeleteEmptyDirData: true}, ""},
		{"pod with no phase is treated as running",
			storagePod("p", drainPodSpec{controllerKind: "ReplicaSet"}, emptyDirVolume("cache")), DrainOptions{DeleteEmptyDirData: true}, findings.Risk},

		{"a DaemonSet kind outside apps/v1 is no DaemonSet to kubectl: the volume is refused",
			foreignDaemonSetPod(), DrainOptions{}, findings.Blocked},
		{"a DaemonSet kind outside apps/v1 with the flag",
			foreignDaemonSetPod(), DrainOptions{DeleteEmptyDirData: true}, findings.Risk},

		// kubectl checks local storage before the controller check.
		{"pod with no controller and emptyDir, no flags: kubectl names the volume first",
			storagePod("p", drainPodSpec{phase: corev1.PodRunning}, emptyDirVolume("cache")), DrainOptions{}, findings.Blocked},
		{"pod with no controller and emptyDir, only the volume flag: the volume is deleted once --force is added",
			storagePod("p", drainPodSpec{phase: corev1.PodRunning}, emptyDirVolume("cache")), DrainOptions{DeleteEmptyDirData: true}, findings.Risk},
		{"pod with no controller and emptyDir, both flags",
			storagePod("p", drainPodSpec{phase: corev1.PodRunning}, emptyDirVolume("cache")), DrainOptions{DeleteEmptyDirData: true, Force: true}, findings.Risk},
		{"owner reference that is not the controller counts as none",
			storagePod("p", drainPodSpec{controllerKind: "ReplicaSet", notController: true, phase: corev1.PodRunning}, emptyDirVolume("cache")), DrainOptions{}, findings.Blocked},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := storageFindings([]corev1.Pod{tt.pod}, tt.drain)
			if tt.want == "" {
				if len(got) != 0 {
					t.Fatalf("findings = %+v, want none", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("got %d findings, want 1: %+v", len(got), got)
			}
			if got[0].Severity != tt.want {
				t.Errorf("severity = %s, want %s", got[0].Severity, tt.want)
			}
			if got[0].RuleID != PodLocalStorageID {
				t.Errorf("rule = %q, want %q", got[0].RuleID, PodLocalStorageID)
			}
			if ev := got[0].Evidence.(findings.LocalStorageEvidence); ev.DeleteEmptyDirData != tt.drain.DeleteEmptyDirData {
				t.Errorf("evidence delete_emptydir_data = %v, want %v", ev.DeleteEmptyDirData, tt.drain.DeleteEmptyDirData)
			}
		})
	}
}

func TestPodLocalStorageSelection(t *testing.T) {
	labelled := storagePod("p", running, emptyDirVolume("cache"))
	labelled.Labels = map[string]string{"app": "web"}

	t.Run("a pod on a node that is not a target is not reported", func(t *testing.T) {
		if got := storageFindings([]corev1.Pod{labelled}, DrainOptions{}, "worker-02"); len(got) != 0 {
			t.Errorf("findings = %+v, want none", got)
		}
	})
	t.Run("no targets", func(t *testing.T) {
		if got := (PodLocalStorage{}).Evaluate(Input{Snapshot: storageSnapshot(labelled)}); len(got) != 0 {
			t.Errorf("findings = %+v, want none", got)
		}
	})
	t.Run("nil snapshot", func(t *testing.T) {
		if got := (PodLocalStorage{}).Evaluate(Input{Targets: []string{"worker-01"}}); got != nil {
			t.Errorf("findings = %+v, want nil", got)
		}
	})
	t.Run("a pod --pod-selector does not match is not touched by the drain", func(t *testing.T) {
		got := storageFindings([]corev1.Pod{labelled}, DrainOptions{PodSelector: mustSelector(t, "app=other")})
		if len(got) != 0 {
			t.Errorf("findings = %+v, want none", got)
		}
	})
	t.Run("a pod --pod-selector matches is reported", func(t *testing.T) {
		got := storageFindings([]corev1.Pod{labelled}, DrainOptions{PodSelector: mustSelector(t, "app=web")})
		if len(got) != 1 {
			t.Errorf("got %d findings, want 1", len(got))
		}
	})
	t.Run("a selector filters pod by pod inside one workload", func(t *testing.T) {
		other := storagePod("q", running, emptyDirVolume("cache"))
		other.Labels = map[string]string{"app": "other"}
		got := storageFindings([]corev1.Pod{labelled, other}, DrainOptions{PodSelector: mustSelector(t, "app=web")})
		if len(got) != 1 || len(got[0].Evidence.(findings.LocalStorageEvidence).Pods) != 1 {
			t.Errorf("findings = %+v, want one finding with one pod", got)
		}
	})
}

// One finding per cause: the workload that owns the pods, or the pod itself
// when nothing controls it. Every pod and volume stays in the evidence.
func TestPodLocalStorageGrouping(t *testing.T) {
	web := func(name, node string, volumes ...corev1.Volume) corev1.Pod {
		p := storagePod(name, running, volumes...)
		p.Spec.NodeName = node
		p.OwnerReferences[0].Name = "web-7f9c"
		return p
	}
	statefulSet := func(ns, name string) corev1.Pod {
		p := storagePod(name, drainPodSpec{controllerKind: "StatefulSet", phase: corev1.PodRunning}, emptyDirVolume("scratch"))
		p.Namespace = ns
		p.OwnerReferences[0].Name = "db"
		return p
	}

	t.Run("pods of one Deployment on two nodes are one finding", func(t *testing.T) {
		pods := []corev1.Pod{web("web-b", "worker-02", emptyDirVolume("cache")), web("web-a", "worker-01", emptyDirVolume("cache")), web("web-c", "worker-01", emptyDirVolume("cache"))}
		got := storageFindings(pods, DrainOptions{}, "worker-01", "worker-02")
		if len(got) != 1 {
			t.Fatalf("got %d findings, want 1: %+v", len(got), got)
		}
		f := got[0]
		if want := (findings.ResourceRef{Kind: "Deployment", Namespace: "shop", Name: "web"}); f.Resource != want {
			t.Errorf("resource = %v, want %v", f.Resource, want)
		}
		want := findings.LocalStorageEvidence{
			OwnerKind: "Deployment", OwnerName: "web",
			Nodes: []string{"worker-01", "worker-02"},
			Pods: []findings.LocalStoragePod{
				{Namespace: "shop", Name: "web-a", Node: "worker-01", Volumes: []string{"cache"}},
				{Namespace: "shop", Name: "web-b", Node: "worker-02", Volumes: []string{"cache"}},
				{Namespace: "shop", Name: "web-c", Node: "worker-01", Volumes: []string{"cache"}},
			},
		}
		if !evidenceEqual(f.Evidence.(findings.LocalStorageEvidence), want) {
			t.Errorf("evidence = %+v\nwant %+v", f.Evidence, want)
		}
	})
	t.Run("only pods with emptyDir are listed", func(t *testing.T) {
		got := storageFindings([]corev1.Pod{web("web-a", "worker-01", emptyDirVolume("cache")), web("web-b", "worker-01", claimVolume("data"))}, DrainOptions{})
		if len(got) != 1 || len(got[0].Evidence.(findings.LocalStorageEvidence).Pods) != 1 {
			t.Errorf("findings = %+v, want one finding with the one pod that has emptyDir", got)
		}
	})
	t.Run("volumes are the emptyDir ones, sorted, whatever else the pod mounts", func(t *testing.T) {
		got := storageFindings([]corev1.Pod{web("web-a", "worker-01",
			emptyDirVolume("zeta"), configMapVolume("cfg"), memoryEmptyDir("alpha"), hostPathVolume("logs"), emptyDirVolume("mid"))}, DrainOptions{})
		if len(got) != 1 {
			t.Fatalf("got %d findings", len(got))
		}
		if v := got[0].Evidence.(findings.LocalStorageEvidence).Pods[0].Volumes; !slices.Equal(v, []string{"alpha", "mid", "zeta"}) {
			t.Errorf("volumes = %v, want [alpha mid zeta]", v)
		}
	})
	t.Run("workloads with the same name in two namespaces are two findings", func(t *testing.T) {
		got := storageFindings([]corev1.Pod{statefulSet("a", "db-0"), statefulSet("b", "db-0")}, DrainOptions{})
		if len(got) != 2 {
			t.Fatalf("got %d findings, want 2: %+v", len(got), got)
		}
	})
	t.Run("two pods of one StatefulSet are one finding", func(t *testing.T) {
		got := storageFindings([]corev1.Pod{statefulSet("a", "db-0"), statefulSet("a", "db-1")}, DrainOptions{})
		if len(got) != 1 || got[0].Resource.Kind != "StatefulSet" || len(got[0].Evidence.(findings.LocalStorageEvidence).Pods) != 2 {
			t.Fatalf("findings = %+v, want one StatefulSet finding with 2 pods", got)
		}
	})
	t.Run("a pod with no controller is its own finding, on the pod", func(t *testing.T) {
		a := storagePod("a", drainPodSpec{phase: corev1.PodRunning}, emptyDirVolume("cache"))
		b := storagePod("b", drainPodSpec{phase: corev1.PodRunning}, emptyDirVolume("cache"))
		got := storageFindings([]corev1.Pod{a, b}, DrainOptions{})
		if len(got) != 2 {
			t.Fatalf("got %d findings, want 2", len(got))
		}
		for _, f := range got {
			ev := f.Evidence.(findings.LocalStorageEvidence)
			if f.Resource.Kind != "Pod" || ev.OwnerKind != "" || ev.OwnerName != "" || len(ev.Pods) != 1 {
				t.Errorf("finding = %+v", f)
			}
		}
	})
	t.Run("the result does not depend on the order of the pods", func(t *testing.T) {
		pods := []corev1.Pod{web("web-b", "worker-02", emptyDirVolume("cache")), statefulSet("a", "db-0"), web("web-a", "worker-01", emptyDirVolume("cache"))}
		reversed := slices.Clone(pods)
		slices.Reverse(reversed)
		first := storageFindings(pods, DrainOptions{}, "worker-01", "worker-02")
		second := storageFindings(reversed, DrainOptions{}, "worker-01", "worker-02")
		findings.Sort(first)
		findings.Sort(second)
		a, _ := json.Marshal(first)
		b, _ := json.Marshal(second)
		if string(a) != string(b) {
			t.Errorf("order changed the result:\n%s\n%s", a, b)
		}
	})
}

func evidenceEqual(a, b findings.LocalStorageEvidence) bool {
	x, _ := json.Marshal(a)
	y, _ := json.Marshal(b)
	return string(x) == string(y)
}

// The text says what kubectl does with the flags in force, and the JSON keys
// are a contract.
func TestPodLocalStorageTextAndSchema(t *testing.T) {
	pod := storagePod("web-a", running, emptyDirVolume("cache"))

	blocked := storageFindings([]corev1.Pod{pod}, DrainOptions{})[0]
	if !strings.Contains(blocked.Description, "--delete-emptydir-data") || !strings.Contains(blocked.Description, "cannot delete Pods with local storage") {
		t.Errorf("BLOCKED description does not say what kubectl does: %s", blocked.Description)
	}
	if !strings.Contains(blocked.Remediation, "--delete-emptydir-data") {
		t.Errorf("BLOCKED remediation does not name the flag: %s", blocked.Remediation)
	}
	risk := storageFindings([]corev1.Pod{pod}, DrainOptions{DeleteEmptyDirData: true})[0]
	if !strings.Contains(risk.Description, "lost") {
		t.Errorf("RISK description does not say the data is lost: %s", risk.Description)
	}
	if strings.Contains(risk.Remediation, "drain with --delete-emptydir-data") {
		t.Errorf("RISK remediation tells the operator to pass a flag already given: %s", risk.Remediation)
	}

	raw, err := json.Marshal(blocked.Evidence)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"owner_kind":"Deployment","owner_name":"web","delete_emptydir_data":false,"nodes":["worker-01"],"pods":[{"namespace":"shop","name":"web-a","node":"worker-01","volumes":["cache"]}]}`
	if string(raw) != want {
		t.Errorf("evidence JSON = %s\nwant %s", raw, want)
	}

	naked := storagePod("tools", drainPodSpec{phase: corev1.PodRunning}, emptyDirVolume("cache"))
	raw, err = json.Marshal(storageFindings([]corev1.Pod{naked}, DrainOptions{DeleteEmptyDirData: true})[0].Evidence)
	if err != nil {
		t.Fatal(err)
	}
	want = `{"delete_emptydir_data":true,"nodes":["worker-01"],"pods":[{"namespace":"shop","name":"tools","node":"worker-01","volumes":["cache"]}]}`
	if string(raw) != want {
		t.Errorf("evidence JSON of a pod with no controller = %s\nwant %s", raw, want)
	}
}

// The fixture covers each way a pod meets the filter: two workloads, a
// StatefulSet with two volumes, a pod with no controller, and the pods that
// are not reported (no emptyDir, finished, DaemonSet, static, other node).
func TestPodLocalStorageFixture(t *testing.T) {
	s := snapshotFromFixture(t, "pod-local-storage.yaml")
	targets := []string{"worker-01", "worker-02"}

	type want struct {
		resource string
		pods     []string
		volumes  map[string][]string
		nodes    []string
	}
	wants := []want{
		{"Deployment/shop/mem", []string{"mem-1"}, map[string][]string{"mem-1": {"tmp"}}, []string{"worker-01"}},
		{"Deployment/shop/web", []string{"web-a1", "web-a2", "web-b1"}, map[string][]string{"web-a1": {"cache"}}, []string{"worker-01", "worker-02"}},
		{"Pod/tools/debug", []string{"debug"}, map[string][]string{"debug": {"scratch"}}, []string{"worker-01"}},
		{"StatefulSet/shop/db", []string{"db-0"}, map[string][]string{"db-0": {"a", "z"}}, []string{"worker-02"}},
	}
	for _, tt := range []struct {
		name  string
		drain DrainOptions
		sev   findings.Severity
	}{
		{"without --delete-emptydir-data", DrainOptions{IgnoreDaemonSets: true}, findings.Blocked},
		{"with --delete-emptydir-data", DrainOptions{IgnoreDaemonSets: true, DeleteEmptyDirData: true}, findings.Risk},
		{"the other flags make no difference", DrainOptions{Force: true, DisableEviction: true}, findings.Blocked},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := PodLocalStorage{}.Evaluate(Input{Snapshot: s, Targets: targets, Drain: tt.drain})
			findings.Sort(got)
			if len(got) != len(wants) {
				t.Fatalf("got %d findings, want %d: %+v", len(got), len(wants), got)
			}
			slices.SortFunc(got, func(a, b findings.Finding) int { return strings.Compare(a.Resource.String(), b.Resource.String()) })
			for i, w := range wants {
				f := got[i]
				if f.Resource.String() != w.resource || f.Severity != tt.sev {
					t.Errorf("finding %d = %s %s, want %s %s", i, f.Severity, f.Resource, tt.sev, w.resource)
					continue
				}
				ev := f.Evidence.(findings.LocalStorageEvidence)
				var names []string
				for _, p := range ev.Pods {
					names = append(names, p.Name)
					if v, ok := w.volumes[p.Name]; ok && !slices.Equal(p.Volumes, v) {
						t.Errorf("%s: pod %s volumes = %v, want %v", w.resource, p.Name, p.Volumes, v)
					}
				}
				if !slices.Equal(names, w.pods) || !slices.Equal(ev.Nodes, w.nodes) {
					t.Errorf("%s: pods %v nodes %v, want pods %v nodes %v", w.resource, names, ev.Nodes, w.pods, w.nodes)
				}
			}
		})
	}
}
