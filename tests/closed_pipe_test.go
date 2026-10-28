//go:build !windows

package tests

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// rfc3339 matches the snapshot time, which the report prints in whole seconds
// in the header and again in the closing disclaimer. Two runs that straddle a
// second boundary print different values, so a comparison of their stdout masks
// it; the rest of the report is still compared byte for byte.
var rfc3339 = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`)

func maskTimestamp(out string) string {
	return rfc3339.ReplaceAllString(out, "<time>")
}

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
					return ee.ExitCode(), maskTimestamp(stdout.String())
				} else if err != nil {
					t.Fatal(err)
				}
				return 0, maskTimestamp(stdout.String())
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

func TestMaskTimestamp(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{"header", "Timestamp: 2026-10-28T09:00:01Z\n", "Timestamp: <time>\n"},
		{"other second, same result", "Timestamp: 2026-10-28T09:00:02Z\n", "Timestamp: <time>\n"},
		{"disclaimer", "state at 2026-10-28T09:00:01Z and does not", "state at <time> and does not"},
		{"every occurrence", "2026-10-28T09:00:01Z 2026-10-28T09:00:01Z", "<time> <time>"},
		{"a date alone is kept", "2026-10-28 and v1.2.3", "2026-10-28 and v1.2.3"},
		{"no timestamp", "body\n", "body\n"},
		{"empty", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := maskTimestamp(tc.in); got != tc.want {
				t.Errorf("maskTimestamp(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
