//go:build !windows

package tests

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// run starts the binary and returns its exit code, stdout and stderr.
func run(t *testing.T, bin string, args ...string) (int, string, string) {
	t.Helper()
	cmd := exec.Command(bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	code := 0
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		code = ee.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return code, stdout.String(), stderr.String()
}

// A selector that does not parse ends the run with exit 2 before any cluster
// call: the kubeconfig does not exist, so reaching the cluster would give a
// different message.
func TestInvalidPodSelectorExitsWith2(t *testing.T) {
	bin := buildBinary(t)
	for _, debug := range []bool{false, true} {
		args := []string{"--nodes", "n1", "--kubeconfig", "/no/such/kubeconfig", "--pod-selector", "app in (web"}
		if debug {
			args = append(args, "--debug")
		}
		code, stdout, stderr := run(t, bin, args...)
		if code != 2 || stdout != "" || !strings.Contains(stderr, `invalid --pod-selector "app in (web"`) {
			t.Errorf("debug=%t: exit %d, stdout %q, stderr %q; want exit 2, no stdout and an --pod-selector error", debug, code, stdout, stderr)
		}
		if strings.Contains(stderr, "kubeconfig") {
			t.Errorf("debug=%t: the run went on to load the kubeconfig:\n%s", debug, stderr)
		}
	}
}

// The five drain flags are known to the binary and listed in --help.
func TestHelpListsTheDrainFlags(t *testing.T) {
	bin := buildBinary(t)
	code, stdout, _ := run(t, bin, "--help")
	if code != 0 {
		t.Fatalf("--help exit code = %d", code)
	}
	for _, flag := range []string{"--ignore-daemonsets", "--delete-emptydir-data", "--force", "--pod-selector", "--disable-eviction"} {
		if !strings.Contains(stdout, flag) {
			t.Errorf("--help does not list %s", flag)
		}
	}
}

// With the drain flags and an unknown one next to them, the unknown flag is
// the error: nothing is silently ignored.
func TestUnknownFlagIsStillRejected(t *testing.T) {
	bin := buildBinary(t)
	code, _, stderr := run(t, bin, "--nodes", "n1", "--force", "--grace-period", "10")
	if code != 2 || !strings.Contains(stderr, "grace-period") {
		t.Errorf("exit %d, stderr %q; want exit 2 naming --grace-period", code, stderr)
	}
}
