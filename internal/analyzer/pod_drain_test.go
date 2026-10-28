package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Veinar/draincheck/internal/inventory"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// drainOwners resolves the owners the cases below use: a Deployment "web"
// behind the ReplicaSet "web-7f9c".
func drainOwners() *inventory.OwnerResolver {
	controller := true
	rs := appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
		Namespace:       "shop",
		Name:            "web-7f9c",
		OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "Deployment", Name: "web", Controller: &controller}},
	}}
	return inventory.NewOwnerResolver(inventory.NewSnapshot(nil, nil, []appsv1.ReplicaSet{rs}, nil, time.Time{}))
}

// drainPod builds a pod with the given controller kind ("" for none), phase and
// the flags that change how a drain treats it.
type drainPodSpec struct {
	controllerKind string
	notController  bool
	phase          corev1.PodPhase
	terminating    bool
	mirror         bool
	nodeOwner      bool
}

func (p drainPodSpec) pod() corev1.Pod {
	controller := !p.notController
	meta := metav1.ObjectMeta{Namespace: "shop", Name: "p"}
	switch p.controllerKind {
	case "":
	case "ReplicaSet":
		meta.OwnerReferences = []metav1.OwnerReference{{Kind: "ReplicaSet", Name: "web-7f9c", Controller: &controller}}
	default:
		meta.OwnerReferences = []metav1.OwnerReference{{Kind: p.controllerKind, Name: "x", Controller: &controller}}
	}
	if p.nodeOwner {
		meta.OwnerReferences = []metav1.OwnerReference{{Kind: "Node", Name: "worker-01", Controller: &controller}}
	}
	if p.mirror {
		meta.Annotations = map[string]string{mirrorPodAnnotation: "abc"}
	}
	if p.terminating {
		now := metav1.Now()
		meta.DeletionTimestamp = &now
	}
	return corev1.Pod{ObjectMeta: meta, Status: corev1.PodStatus{Phase: p.phase}}
}

// The classification is the one place that says what a drain does with a pod.
// Every rule asks it, so the same pod is never in one finding and absent from
// the next.
func TestClassifyPod(t *testing.T) {
	tests := []struct {
		name           string
		pod            drainPodSpec
		fate           drainFate
		bypassesBudget bool
		consultsBudget bool
		// ownership is the class POD_OWNERSHIP reports, regardless of fate.
		class podClass
	}{
		{"running Deployment pod is evicted and rescheduled",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning}, fateEvictedRescheduled, false, true, classManaged},
		{"pod with no phase reported is treated as running",
			drainPodSpec{controllerKind: "ReplicaSet"}, fateEvictedRescheduled, false, true, classManaged},
		{"StatefulSet pod is evicted and rescheduled",
			drainPodSpec{controllerKind: "StatefulSet", phase: corev1.PodRunning}, fateEvictedRescheduled, false, true, classManaged},
		{"Job pod is evicted and rescheduled",
			drainPodSpec{controllerKind: "Job", phase: corev1.PodRunning}, fateEvictedRescheduled, false, true, classManaged},
		{"pod with no controller is evicted and not recreated",
			drainPodSpec{phase: corev1.PodRunning}, fateEvictedUnmanaged, false, true, classUnmanaged},
		{"owner reference that is not the controller counts as no controller",
			drainPodSpec{controllerKind: "ReplicaSet", notController: true, phase: corev1.PodRunning}, fateEvictedUnmanaged, false, true, classUnmanaged},
		{"pod owned by a Node without the mirror annotation is evicted and rescheduled",
			drainPodSpec{nodeOwner: true, phase: corev1.PodRunning}, fateEvictedRescheduled, false, true, classManaged},
		{"DaemonSet pod is left alone",
			drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodRunning}, fateLeftAlone, false, false, classDaemonSet},
		{"DaemonSet reference that is not the controller is not a DaemonSet pod",
			drainPodSpec{controllerKind: "DaemonSet", notController: true, phase: corev1.PodRunning}, fateEvictedUnmanaged, false, true, classUnmanaged},
		{"static pod is left alone",
			drainPodSpec{mirror: true, phase: corev1.PodRunning}, fateLeftAlone, false, false, classStatic},
		{"static pod with a Node owner is left alone",
			drainPodSpec{mirror: true, nodeOwner: true, phase: corev1.PodRunning}, fateLeftAlone, false, false, classStatic},
		{"static pod wins over a DaemonSet owner",
			drainPodSpec{mirror: true, controllerKind: "DaemonSet", phase: corev1.PodRunning}, fateLeftAlone, false, false, classStatic},

		{"Succeeded pod is finished",
			drainPodSpec{controllerKind: "Job", phase: corev1.PodSucceeded}, fateFinished, true, false, classFinished},
		{"Failed pod is finished",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodFailed}, fateFinished, true, false, classFinished},
		{"finished DaemonSet pod is finished, not left alone",
			drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodFailed}, fateFinished, true, false, classFinished},
		{"finished static pod is finished",
			drainPodSpec{mirror: true, phase: corev1.PodSucceeded}, fateFinished, true, false, classFinished},
		{"finished pod that is also terminating is finished",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodFailed, terminating: true}, fateFinished, true, false, classFinished},
		{"finished pod with no controller is finished",
			drainPodSpec{phase: corev1.PodSucceeded}, fateFinished, true, false, classFinished},

		{"terminating Deployment pod is already leaving",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning, terminating: true}, fateLeaving, true, false, classManaged},
		{"terminating pod with no controller is already leaving",
			drainPodSpec{phase: corev1.PodRunning, terminating: true}, fateLeaving, true, false, classUnmanaged},
		{"terminating DaemonSet pod stays a DaemonSet pod for ownership",
			drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodRunning, terminating: true}, fateLeaving, true, false, classDaemonSet},
		{"terminating static pod stays a static pod for ownership",
			drainPodSpec{mirror: true, phase: corev1.PodRunning, terminating: true}, fateLeaving, true, false, classStatic},
		{"terminating Pending pod is already leaving",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodPending, terminating: true}, fateLeaving, true, false, classManaged},

		{"Pending pod is evicted and rescheduled but bypasses budgets",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodPending}, fateEvictedRescheduled, true, false, classManaged},
		{"Pending pod with no controller is evicted and not recreated",
			drainPodSpec{phase: corev1.PodPending}, fateEvictedUnmanaged, true, false, classUnmanaged},
		{"Pending DaemonSet pod is left alone",
			drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodPending}, fateLeftAlone, true, false, classDaemonSet},
		{"Unknown phase is treated as running",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodUnknown}, fateEvictedRescheduled, false, true, classManaged},
	}
	owners := drainOwners()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := classifyPod(tt.pod.pod(), owners, DrainOptions{})
			if got := d.fate(); got != tt.fate {
				t.Errorf("fate = %v, want %v", got, tt.fate)
			}
			if got := d.bypassesBudget(); got != tt.bypassesBudget {
				t.Errorf("bypassesBudget = %v, want %v", got, tt.bypassesBudget)
			}
			if got := d.consultsBudget(); got != tt.consultsBudget {
				t.Errorf("consultsBudget = %v, want %v", got, tt.consultsBudget)
			}
			if d.class != tt.class {
				t.Errorf("class = %v, want %v", d.class, tt.class)
			}
		})
	}
}

// --disable-eviction makes kubectl delete the pods instead of evicting them, so
// no budget is consulted for any pod. Nothing else about the pod changes: the
// drain still acts on it, and the other rules judge it as before.
func TestClassifyPodWithDisabledEviction(t *testing.T) {
	tests := []struct {
		name string
		pod  drainPodSpec
	}{
		{"running Deployment pod", drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning}},
		{"pod with no phase", drainPodSpec{controllerKind: "ReplicaSet"}},
		{"pod with no controller", drainPodSpec{phase: corev1.PodRunning}},
		{"Pending pod", drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodPending}},
		{"terminating pod", drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning, terminating: true}},
		{"finished pod", drainPodSpec{controllerKind: "Job", phase: corev1.PodSucceeded}},
		{"DaemonSet pod", drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodRunning}},
		{"static pod", drainPodSpec{mirror: true, phase: corev1.PodRunning}},
	}
	owners := drainOwners()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := tt.pod.pod()
			with := classifyPod(pod, owners, DrainOptions{DisableEviction: true})
			without := classifyPod(pod, owners, DrainOptions{})

			if with.consultsBudget() {
				t.Error("consultsBudget = true with --disable-eviction, want false")
			}
			if with.fate() != without.fate() || with.class != without.class || with.bypassesBudget() != without.bypassesBudget() {
				t.Errorf("fate/class/bypassesBudget = %v/%v/%v, want %v/%v/%v as without the flag",
					with.fate(), with.class, with.bypassesBudget(), without.fate(), without.class, without.bypassesBudget())
			}
		})
	}
}

// With the flag, a pod that --pod-selector leaves out stays out, and a pod it
// keeps still consults no budget.
func TestClassifyPodWithDisabledEvictionAndSelector(t *testing.T) {
	sel, err := labels.Parse("app=web")
	if err != nil {
		t.Fatal(err)
	}
	pod := drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning}.pod()
	pod.Labels = map[string]string{"app": "web"}
	other := pod.DeepCopy()
	other.Labels = map[string]string{"app": "db"}

	drain := DrainOptions{DisableEviction: true, PodSelector: sel}
	if d := classifyPod(pod, drainOwners(), drain); !d.selected || d.consultsBudget() || d.fate() != fateEvictedRescheduled {
		t.Errorf("matching pod: selected=%v consultsBudget=%v fate=%v", d.selected, d.consultsBudget(), d.fate())
	}
	if d := classifyPod(*other, drainOwners(), drain); d.selected || d.fate() != fateNotSelected {
		t.Errorf("non-matching pod: selected=%v fate=%v", d.selected, d.fate())
	}
}

// Every fate has a reason an operator can read in a debug line, and the
// reasons differ.
func TestDrainFateString(t *testing.T) {
	fates := []drainFate{fateNotSelected, fateFinished, fateLeaving, fateLeftAlone, fateEvictedUnmanaged, fateEvictedRescheduled}
	seen := map[string]drainFate{}
	for _, f := range fates {
		s := f.String()
		if s == "" || s == "unknown" {
			t.Errorf("fate %d has no reason", f)
		}
		if other, dup := seen[s]; dup {
			t.Errorf("fates %d and %d share the reason %q", other, f, s)
		}
		seen[s] = f
	}
	if got := drainFate(99).String(); got != "unknown" {
		t.Errorf("out-of-range fate = %q, want unknown", got)
	}
}

// A pod that --pod-selector does not match is not listed by kubectl drain, so
// it has no fate of its own whatever it is: not evicted, not counted, not
// checked. Its class stays what it is.
func TestClassifyPodWithSelector(t *testing.T) {
	selector := DrainOptions{PodSelector: mustSelector(t, "app=web")}
	tests := []struct {
		name     string
		pod      drainPodSpec
		labels   map[string]string
		selected bool
		fate     drainFate
		consults bool
	}{
		{"matching pod is judged as without a selector",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning}, map[string]string{"app": "web"}, true, fateEvictedRescheduled, true},
		{"unmatched running pod is not selected",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning}, map[string]string{"app": "db"}, false, fateNotSelected, false},
		{"unlabeled pod is not selected",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning}, nil, false, fateNotSelected, false},
		{"unmatched pod with no controller is not selected",
			drainPodSpec{phase: corev1.PodRunning}, nil, false, fateNotSelected, false},
		{"unmatched DaemonSet pod is not selected, not left alone",
			drainPodSpec{controllerKind: "DaemonSet", phase: corev1.PodRunning}, nil, false, fateNotSelected, false},
		{"unmatched finished pod is not selected",
			drainPodSpec{controllerKind: "Job", phase: corev1.PodSucceeded}, nil, false, fateNotSelected, false},
		{"unmatched terminating pod is not selected",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning, terminating: true}, nil, false, fateNotSelected, false},
		{"matching terminating pod is still leaving",
			drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning, terminating: true}, map[string]string{"app": "web"}, true, fateLeaving, false},
	}
	owners := drainOwners()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := tt.pod.pod()
			pod.Labels = tt.labels
			d := classifyPod(pod, owners, selector)
			if d.selected != tt.selected {
				t.Errorf("selected = %v, want %v", d.selected, tt.selected)
			}
			if got := d.fate(); got != tt.fate {
				t.Errorf("fate = %v, want %v", got, tt.fate)
			}
			if got := d.consultsBudget(); got != tt.consults {
				t.Errorf("consultsBudget = %v, want %v", got, tt.consults)
			}
			if without := classifyPod(pod, owners, DrainOptions{}); d.class != without.class {
				t.Errorf("class = %v, want %v (the selector must not change what the pod is)", d.class, without.class)
			}
		})
	}
}

// The classification reads the pod and the snapshot only; it never changes the pod.
func TestClassifyPodDoesNotMutate(t *testing.T) {
	pod := drainPodSpec{controllerKind: "ReplicaSet", phase: corev1.PodRunning, terminating: true}.pod()
	before := pod.DeepCopy()
	classifyPod(pod, drainOwners(), DrainOptions{})
	if pod.Name != before.Name || pod.DeletionTimestamp == nil || len(pod.OwnerReferences) != len(before.OwnerReferences) {
		t.Error("classifyPod changed the pod")
	}
}

// No rule decides on its own which pods a drain acts on: the phase, the
// deletion timestamp and the mirror annotation are read in pod_drain.go only.
// A rule that filters pods by itself brings back the disagreement between
// rules that classifyPod exists to end.
func TestRulesDoNotFilterPodsThemselves(t *testing.T) {
	forbidden := []string{"DeletionTimestamp", "PodSucceeded", "PodFailed", "PodPending", "PodRunning", "mirrorPodAnnotation", "Status.Phase"}
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || f == "pod_drain.go" {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		checked++
		for n, line := range strings.Split(string(src), "\n") {
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			for _, word := range forbidden {
				if strings.Contains(line, word) {
					t.Errorf("%s:%d reads %s; ask classifyPod instead", f, n+1, word)
				}
			}
		}
	}
	if checked < 10 {
		t.Fatalf("checked only %d files, the glob is wrong", checked)
	}
}
