//go:build !windows

package tests

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// closedPipe returns the write end of a pipe whose reader is already gone.
func closedPipe(t *testing.T) *os.File {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := r.Close(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	return w
}

// TestClosedPipeKeepsTheExitCode runs the binary with stdout, stderr or both
// piped into a reader that has already exited. A write to the closed pipe must
// not end the process with SIGPIPE (shell code 141): a failed write to stdout
// exits 2, and a failed debug or error write to stderr changes nothing.
func TestClosedPipeKeepsTheExitCode(t *testing.T) {
	bin := buildBinary(t)
	missing := filepath.Join(t.TempDir(), "no-such-kubeconfig")

	cases := []struct {
		name           string
		args           []string
		stdout, stderr bool
		want           int
	}{
		{"stdout closed, output fails", []string{"--version"}, true, false, 2},
		{"stderr closed, failure", []string{"--kubeconfig", missing, "--nodes", "n1"}, false, true, 2},
		{"stderr closed, failure, debug", []string{"--kubeconfig", missing, "--nodes", "n1", "--debug"}, false, true, 2},
		{"both closed, failure, debug", []string{"--kubeconfig", missing, "--nodes", "n1", "--debug"}, true, true, 2},
		{"both closed, output fails", []string{"--version"}, true, true, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(bin, tc.args...)
			if tc.stdout {
				cmd.Stdout = closedPipe(t)
			}
			if tc.stderr {
				cmd.Stderr = closedPipe(t)
			}
			err := cmd.Run()
			got := 0
			var ee *exec.ExitError
			if errors.As(err, &ee) {
				got = ee.ExitCode() // -1 when a signal ended the process
			} else if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("exit code = %d (%v), want %d", got, err, tc.want)
			}
		})
	}
}

// TestClosedStderrKeepsFindingsExitCodeAndReport runs a real analysis with
// --debug and a closed stderr. The exit code (1 with a finding, 0 without) and
// the report on stdout are the same as for a run without --debug.
func TestClosedStderrKeepsFindingsExitCodeAndReport(t *testing.T) {
	bin := buildBinary(t)
	for _, tc := range []struct {
		name    string
		barePod bool
		want    int
	}{
		{"finding", true, 1},
		{"clean", false, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			kubeconfig := writeKubeconfig(t, startFakeAPI(t, tc.barePod))
			run := func(extra ...string) (int, string) {
				cmd := exec.Command(bin, append([]string{"--kubeconfig", kubeconfig, "--nodes", "n1", "--fail-on", "risk"}, extra...)...)
				var stdout strings.Builder
				cmd.Stdout = &stdout
				if len(extra) > 0 {
					cmd.Stderr = closedPipe(t)
				}
				err := cmd.Run()
				var ee *exec.ExitError
				if errors.As(err, &ee) {
					return ee.ExitCode(), stdout.String()
				} else if err != nil {
					t.Fatal(err)
				}
				return 0, stdout.String()
			}
			wantCode, wantOut := run()
			if wantCode != tc.want {
				t.Fatalf("baseline exit code = %d, want %d (fake API or rule changed)", wantCode, tc.want)
			}
			if gotCode, gotOut := run("--debug"); gotCode != wantCode || gotOut != wantOut {
				t.Fatalf("closed stderr with --debug: exit %d, want %d; stdout equal = %t", gotCode, wantCode, gotOut == wantOut)
			}
		})
	}
}
