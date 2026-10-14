package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const twoContextKubeconfig = `apiVersion: v1
kind: Config
current-context: dev
clusters:
- name: c
  cluster: {server: "https://127.0.0.1:6443"}
users:
- name: u
  user: {token: t}
contexts:
- name: dev
  context: {cluster: c, user: u}
- name: prod
  context: {cluster: c, user: u}
`

func writeKubeconfig(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestNewClientsetMissingKubeconfig(t *testing.T) {
	_, err := newClientset(filepath.Join(t.TempDir(), "absent"), "")
	if err == nil {
		t.Fatal("newClientset() error = nil, want error for a missing kubeconfig")
	}
}

func TestNewClientset(t *testing.T) {
	path := writeKubeconfig(t, twoContextKubeconfig)
	tests := []struct {
		name    string
		context string
		wantErr string
	}{
		{"current context", "", ""},
		{"explicit context", "prod", ""},
		{"unknown context", "staging", "staging"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, err := newClientset(path, tt.context)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || client == nil {
				t.Fatalf("newClientset() = %v, %v", client, err)
			}
		})
	}
}

func TestNewClientsetMalformedKubeconfig(t *testing.T) {
	_, err := newClientset(writeKubeconfig(t, "not: [valid"), "")
	if err == nil {
		t.Fatal("newClientset() error = nil, want error for malformed kubeconfig")
	}
}
