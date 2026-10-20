package analyzer

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Veinar/draincheck/internal/inventory"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

// snapshotFromFixture loads testdata/analyzer/<name> into a fake clientset and
// collects it, so rules are tested against what the real collector produces.
func snapshotFromFixture(t *testing.T, name string) *inventory.Snapshot {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "analyzer", name))
	if err != nil {
		t.Fatal(err)
	}

	var objs []runtime.Object
	docs := utilyaml.NewYAMLReader(bufio.NewReader(bytes.NewReader(data)))
	for {
		doc, err := docs.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if len(bytes.TrimSpace(doc)) == 0 {
			continue
		}
		obj, _, err := scheme.Codecs.UniversalDeserializer().Decode(doc, nil, nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if pod, ok := obj.(*corev1.Pod); ok {
			healthyUnlessStated(pod)
		}
		objs = append(objs, obj)
	}

	s, err := inventory.NewClientsetCollector(fake.NewClientset(objs...), nil).Collect(context.Background())
	if err != nil {
		t.Fatalf("%s: collect: %v", name, err)
	}
	return s
}

// healthyUnlessStated gives a pod whose fixture has no status the status of a
// healthy one, phase Running and Ready=True: the eviction handler treats a pod
// with no Ready condition as not ready, which no running pod in a real cluster
// is. A pod that states its phase or conditions keeps them.
func healthyUnlessStated(pod *corev1.Pod) {
	if pod.Status.Phase != "" || len(pod.Status.Conditions) > 0 {
		return
	}
	pod.Status.Phase = corev1.PodRunning
	pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
}
