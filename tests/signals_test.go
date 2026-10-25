//go:build !windows

package tests

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestStopSignalEndsTheRunWithExit2 stops a run that waits for a server that
// never answers. SIGINT and SIGTERM both end it with exit 2 and an error line,
// with and without --debug; with --debug the finished line is written too.
func TestStopSignalEndsTheRunWithExit2(t *testing.T) {
	bin := buildBinary(t)
	requested := make(chan struct{}, 1)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		select {
		case requested <- struct{}{}:
		default:
		}
		<-release
	}))
	defer srv.Close()
	defer close(release)
	kubeconfig := writeKubeconfig(t, srv.URL)

	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		for _, debug := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/debug=%t", sig, debug), func(t *testing.T) {
				for len(requested) > 0 {
					<-requested
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				args := []string{"--kubeconfig", kubeconfig, "--nodes", "n1"}
				if debug {
					args = append(args, "--debug")
				}
				cmd := exec.CommandContext(ctx, bin, args...)
				var stdout, stderr bytes.Buffer
				cmd.Stdout, cmd.Stderr = &stdout, &stderr
				if err := cmd.Start(); err != nil {
					t.Fatal(err)
				}
				select {
				case <-requested:
				case <-ctx.Done():
					t.Fatal("draincheck never reached the server")
				}
				if err := cmd.Process.Signal(sig); err != nil {
					t.Fatal(err)
				}
				err := cmd.Wait()
				var ee *exec.ExitError
				if !errors.As(err, &ee) || ee.ExitCode() != 2 {
					t.Fatalf("err = %v, want exit 2\nstderr:\n%s", err, &stderr)
				}
				if stdout.Len() != 0 {
					t.Fatalf("stdout = %q, want empty", &stdout)
				}
				if !strings.Contains(stderr.String(), "error:") {
					t.Fatalf("want an error line, got:\n%s", &stderr)
				}
				if got := strings.Contains(stderr.String(), "msg=finished exit_code=2"); got != debug {
					t.Fatalf("finished line present = %t, want %t\nstderr:\n%s", got, debug, &stderr)
				}
			})
		}
	}
}
