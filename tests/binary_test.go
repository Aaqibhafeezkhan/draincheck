//go:build !windows

package tests

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// buildBinary returns the binary built by TestMain.
func buildBinary(t *testing.T) string {
	t.Helper()
	return binPath
}

// writeKubeconfig writes a kubeconfig without credentials that points at
// server, under the test's temp dir.
func writeKubeconfig(t *testing.T, server string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "kubeconfig")
	cfg := fmt.Sprintf(`apiVersion: v1
kind: Config
clusters:
- name: test
  cluster:
    server: %s
contexts:
- name: test
  context:
    cluster: test
    user: test
current-context: test
users:
- name: test
  user: {}
`, server)
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
