package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRootVersionFlag(t *testing.T) {
	cmd := newRootCmd("v1.2.3")
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
