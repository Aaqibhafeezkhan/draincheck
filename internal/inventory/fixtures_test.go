package inventory

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes/scheme"
)

// loadFixture decodes every document of testdata/inventory/<name> into objects.
func loadFixture(t *testing.T, name string) []runtime.Object {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "inventory", name))
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
	return objs
}
