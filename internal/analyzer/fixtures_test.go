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
		objs = append(objs, obj)
	}

	s, err := inventory.NewClientsetCollector(fake.NewClientset(objs...), nil).Collect(context.Background())
	if err != nil {
		t.Fatalf("%s: collect: %v", name, err)
	}
	return s
}
