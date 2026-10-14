package main

import (
	"path/filepath"
	"testing"
)

func TestNewClientsetMissingKubeconfig(t *testing.T) {
	_, err := newClientset(filepath.Join(t.TempDir(), "absent"), "")
	if err == nil {
		t.Fatal("newClientset() error = nil, want error for a missing kubeconfig")
	}
}
