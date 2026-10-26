package main

import (
	"bytes"
	"context"
	"reflect"
	"strings"
	"testing"
)

func TestRootFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    options
		wantErr string
	}{
		{
			name: "defaults with nodes",
			args: []string{"--nodes", "worker-02"},
			want: options{Nodes: []string{"worker-02"}, Output: "text", FailOn: "blocked"},
		},
		{
			name: "all flags",
			args: []string{"--nodes", "worker-02,worker-04", "--kubeconfig", "/tmp/kc", "--context", "prod", "--output", "json"},
			want: options{Nodes: []string{"worker-02", "worker-04"}, Kubeconfig: "/tmp/kc", Context: "prod", Output: "json", FailOn: "blocked"},
		},
		{
			name: "verbose",
			args: []string{"--nodes", "worker-02", "--verbose"},
			want: options{Nodes: []string{"worker-02"}, Output: "text", FailOn: "blocked", Verbose: true},
		},
		{
			name: "debug",
			args: []string{"--nodes", "worker-02", "--debug"},
			want: options{Nodes: []string{"worker-02"}, Output: "text", FailOn: "blocked", Debug: true},
		},
		{
			name: "wildcard selects all nodes",
			args: []string{"--nodes", "*"},
			want: options{AllNodes: true, Output: "text", FailOn: "blocked"},
		},
		{
			name:    "wildcard mixed with names",
			args:    []string{"--nodes", "*,worker-02"},
			wantErr: "'*'",
		},
		{
			name: "whitespace around names is trimmed",
			args: []string{"--nodes", "worker-01, worker-02"},
			want: options{Nodes: []string{"worker-01", "worker-02"}, Output: "text", FailOn: "blocked"},
		},
		{
			name:    "empty node name",
			args:    []string{"--nodes", "worker-01,,worker-02"},
			wantErr: "empty",
		},
		{
			name:    "blank node name",
			args:    []string{"--nodes", "worker-01, "},
			wantErr: "empty",
		},
		{
			name:    "missing nodes",
			args:    []string{},
			wantErr: "--nodes",
		},
		{
			name: "fail-on risk",
			args: []string{"--nodes", "a", "--fail-on", "risk"},
			want: options{Nodes: []string{"a"}, Output: "text", FailOn: "risk"},
		},
		{
			name:    "invalid fail-on",
			args:    []string{"--nodes", "a", "--fail-on", "unknown"},
			wantErr: "--fail-on",
		},
		{
			name: "drain flags",
			args: []string{"--nodes", "a", "--ignore-daemonsets", "--delete-emptydir-data", "--force", "--disable-eviction", "--pod-selector", "app=web"},
			want: options{Nodes: []string{"a"}, Output: "text", FailOn: "blocked",
				IgnoreDaemonSets: true, DeleteEmptyDirData: true, Force: true, DisableEviction: true, PodSelector: "app=web"},
		},
		{
			name: "set-based pod selector",
			args: []string{"--nodes", "a", "--pod-selector", "tier in (front,back),app!=db"},
			want: options{Nodes: []string{"a"}, Output: "text", FailOn: "blocked", PodSelector: "tier in (front,back),app!=db"},
		},
		{
			name: "explicit false and an empty selector are the defaults",
			args: []string{"--nodes", "a", "--force=false", "--ignore-daemonsets=false", "--pod-selector", ""},
			want: options{Nodes: []string{"a"}, Output: "text", FailOn: "blocked"},
		},
		{
			name: "the last --pod-selector wins",
			args: []string{"--nodes", "a", "--pod-selector", "app=web", "--pod-selector", "app=db"},
			want: options{Nodes: []string{"a"}, Output: "text", FailOn: "blocked", PodSelector: "app=db"},
		},
		{
			name: "a blank selector is accepted and selects every pod, as in kubectl",
			args: []string{"--nodes", "a", "--pod-selector", "   "},
			want: options{Nodes: []string{"a"}, Output: "text", FailOn: "blocked", PodSelector: "   "},
		},
		{
			name: "a selector with a prefixed key and a contradiction is valid",
			args: []string{"--nodes", "a", "--pod-selector", "example.com/team=x,app=web,app=db"},
			want: options{Nodes: []string{"a"}, Output: "text", FailOn: "blocked", PodSelector: "example.com/team=x,app=web,app=db"},
		},
		{
			name: "drain flags work with the wildcard",
			args: []string{"--nodes", "*", "--ignore-daemonsets", "--pod-selector", "app=web"},
			want: options{AllNodes: true, Output: "text", FailOn: "blocked", IgnoreDaemonSets: true, PodSelector: "app=web"},
		},
		{
			name:    "a trailing comma is not a selector",
			args:    []string{"--nodes", "a", "--pod-selector", "app=web,"},
			wantErr: "--pod-selector",
		},
		{
			name:    "an empty requirement between commas",
			args:    []string{"--nodes", "a", "--pod-selector", "app=web,,tier=db"},
			wantErr: "--pod-selector",
		},
		{
			name:    "a lone operator",
			args:    []string{"--nodes", "a", "--pod-selector", "="},
			wantErr: "--pod-selector",
		},
		{
			name:    "a value with a space",
			args:    []string{"--nodes", "a", "--pod-selector", "app=a b"},
			wantErr: "--pod-selector",
		},
		{
			name:    "a value that is not a valid label",
			args:    []string{"--nodes", "a", "--pod-selector", "app=web;tier=db"},
			wantErr: "--pod-selector",
		},
		{
			name:    "an unfinished notin",
			args:    []string{"--nodes", "a", "--pod-selector", "app notin"},
			wantErr: "--pod-selector",
		},
		{
			name:    "the error names the selector that was given",
			args:    []string{"--nodes", "a", "--pod-selector", "app in (web"},
			wantErr: `"app in (web"`,
		},
		{
			name:    "a missing --nodes is reported before the selector",
			args:    []string{"--pod-selector", "app in (web"},
			wantErr: "--nodes",
		},
		{
			name:    "invalid pod selector",
			args:    []string{"--nodes", "a", "--pod-selector", "app in (web"},
			wantErr: "--pod-selector",
		},
		{
			name:    "pod selector with a bad operator",
			args:    []string{"--nodes", "a", "--pod-selector", "app==="},
			wantErr: "--pod-selector",
		},
		{
			name:    "invalid output",
			args:    []string{"--nodes", "a", "--output", "yaml"},
			wantErr: "--output",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got options
			cmd := newRootCmd("test", func(_ context.Context, o options) error { got = o; return nil })
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(tt.args)

			err := cmd.Execute()
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Execute() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("options = %+v, want %+v", got, tt.want)
			}
		})
	}
}
