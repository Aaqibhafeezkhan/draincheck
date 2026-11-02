package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestRootVersionFlag(t *testing.T) {
	cmd := newRootCmd("v1.2.3", nil)
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{"--version"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got := out.String(); !strings.Contains(got, "v1.2.3") {
		t.Errorf("output %q does not contain version", got)
	}
}

func TestRootRejectsPositionalArguments(t *testing.T) {
	called := false
	cmd := newRootCmd("dev", func(context.Context, options) error {
		called = true
		return nil
	})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--nodes", "n1", "n2"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute() error = nil, want an error for the extra argument")
	}
	if called {
		t.Error("run was called although an argument was rejected")
	}
	for _, want := range []string{"n2", "--nodes"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
	if got := exitCode(err); got != exitFailure {
		t.Errorf("exit code = %d, want %d", got, exitFailure)
	}
}
