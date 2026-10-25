package main

import (
	"bytes"
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
	_, err := newClientset(filepath.Join(t.TempDir(), "absent"), "", nil)
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
			client, err := newClientset(path, tt.context, nil)
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
	_, err := newClientset(writeKubeconfig(t, "not: [valid"), "", nil)
	if err == nil {
		t.Fatal("newClientset() error = nil, want error for malformed kubeconfig")
	}
}

func TestKubeconfigPaths(t *testing.T) {
	t.Run("explicit path wins over KUBECONFIG", func(t *testing.T) {
		t.Setenv("KUBECONFIG", "/env/one"+string(filepath.ListSeparator)+"/env/two")
		got := kubeconfigPaths("/explicit/config")
		if len(got) != 1 || got[0] != "/explicit/config" {
			t.Errorf("kubeconfigPaths() = %v, want [/explicit/config]", got)
		}
	})
	t.Run("KUBECONFIG lists the files in order", func(t *testing.T) {
		t.Setenv("KUBECONFIG", "/env/one"+string(filepath.ListSeparator)+"/env/two")
		got := kubeconfigPaths("")
		if len(got) != 2 || got[0] != "/env/one" || got[1] != "/env/two" {
			t.Errorf("kubeconfigPaths() = %v, want [/env/one /env/two]", got)
		}
	})
	t.Run("default is never empty", func(t *testing.T) {
		t.Setenv("KUBECONFIG", "")
		if got := kubeconfigPaths(""); len(got) == 0 {
			t.Error("kubeconfigPaths() = [], want the default kubeconfig file")
		}
	})
}

func TestNewClientsetLogsEachStep(t *testing.T) {
	path := writeKubeconfig(t, twoContextKubeconfig)
	var logs bytes.Buffer

	if _, err := newClientset(path, "", newLogger(true, &logs)); err != nil {
		t.Fatalf("newClientset() error = %v", err)
	}

	wantInOrder(t, logs.String(),
		`msg="load kubeconfig" server=127.0.0.1:6443`,
		`msg="build client"`,
	)
}

func TestNewClientsetLogsFailures(t *testing.T) {
	tests := []struct {
		name       string
		kubeconfig func(t *testing.T) string
		context    string
		want       string
	}{
		{"missing file", func(t *testing.T) string { return filepath.Join(t.TempDir(), "absent") }, "", `msg="load kubeconfig failed"`},
		{"malformed file", func(t *testing.T) string { return writeKubeconfig(t, "not: [valid") }, "", `msg="load kubeconfig failed"`},
		{"unknown context", func(t *testing.T) string { return writeKubeconfig(t, twoContextKubeconfig) }, "staging", `msg="load kubeconfig failed"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var logs bytes.Buffer

			_, err := newClientset(tt.kubeconfig(t), tt.context, newLogger(true, &logs))

			if err == nil {
				t.Fatal("newClientset() error = nil, want error")
			}
			wantInOrder(t, logs.String(), tt.want, "error=")
			if strings.Contains(logs.String(), `msg="build client"`) {
				t.Errorf("logged a client build after the load failed:\n%s", logs.String())
			}
		})
	}
}

func TestNewClientsetDebugNeverLogsKubeconfigContents(t *testing.T) {
	const secret = "tok-must-not-leak"
	for name, content := range map[string]string{
		"malformed":  "users: [{user: {token: " + secret + "\n",
		"bad field":  "apiVersion: v1\nkind: Config\nclusters: " + secret + "\n",
		"bad server": "apiVersion: v1\nkind: Config\ncurrent-context: d\nclusters:\n- name: c\n  cluster: {server: \"https://user:" + secret + "@h:1\"}\nusers:\n- name: u\n  user: {token: " + secret + "}\ncontexts:\n- name: d\n  context: {cluster: c, user: u}\n",
	} {
		t.Run(name, func(t *testing.T) {
			var logs bytes.Buffer

			newClientset(writeKubeconfig(t, content), "", newLogger(true, &logs))

			if strings.Contains(logs.String(), secret) {
				t.Errorf("debug log contains kubeconfig contents:\n%s", logs.String())
			}
		})
	}
}
