package report

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTextVerboseGolden(t *testing.T) {
	got := renderVerbose(t, sampleFindings())

	path := filepath.Join("..", "..", "testdata", "report", "text-verbose.golden.txt")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s (rerun with -update if the change is deliberate)\n got:\n%s\nwant:\n%s", path, got, want)
	}
}
