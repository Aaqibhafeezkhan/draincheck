//go:build !windows

package tests

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// binPath is the draincheck binary the tests below run, built once for the
// whole package.
var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "draincheck-tests")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binPath = filepath.Join(dir, "draincheck")
	if out, err := exec.Command("go", "build", "-o", binPath, "../cmd/draincheck").CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "go build: %v\n%s", err, out)
		_ = os.RemoveAll(dir)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
