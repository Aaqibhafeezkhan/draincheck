package main

import (
	"bytes"
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
			want: options{Nodes: []string{"worker-02"}, Output: "text"},
		},
		{
			name: "all flags",
			args: []string{"--nodes", "worker-02,worker-04", "--kubeconfig", "/tmp/kc", "--context", "prod", "--output", "json"},
			want: options{Nodes: []string{"worker-02", "worker-04"}, Kubeconfig: "/tmp/kc", Context: "prod", Output: "json"},
		},
		{
			name: "wildcard selects all nodes",
			args: []string{"--nodes", "*"},
			want: options{AllNodes: true, Output: "text"},
		},
		{
			name:    "wildcard mixed with names",
			args:    []string{"--nodes", "*,worker-02"},
			wantErr: "'*'",
		},
		{
			name: "whitespace around names is trimmed",
			args: []string{"--nodes", "worker-01, worker-02"},
			want: options{Nodes: []string{"worker-01", "worker-02"}, Output: "text"},
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
			name:    "invalid output",
			args:    []string{"--nodes", "a", "--output", "yaml"},
			wantErr: "--output",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got options
			cmd := newRootCmd("test", func(o options) error { got = o; return nil })
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
