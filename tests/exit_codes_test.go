//go:build !windows

package tests

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

// Fixtures for the UNKNOWN-only cluster: two nodes (so the drain of n1 leaves
// room and NO_REMAINING_NODES stays quiet), one pod owned by a ReplicaSet, and
// a PodDisruptionBudget on it whose disruption controller reports SyncFailed.
const (
	fakeTwoNodeList = `{"apiVersion":"v1","kind":"NodeList","items":[` +
		`{"metadata":{"name":"n1"},"status":{"conditions":[{"type":"Ready","status":"True"}]}},` +
		`{"metadata":{"name":"n2"},"status":{"conditions":[{"type":"Ready","status":"True"}]}}]}`
	fakeOwnedPodList = `{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"web-0","namespace":"default","labels":{"app":"web"},` +
		`"ownerReferences":[{"apiVersion":"apps/v1","kind":"ReplicaSet","name":"web-abc","uid":"u1","controller":true}]},` +
		`"spec":{"nodeName":"n1","containers":[{"name":"c","resources":{"requests":{"cpu":"1m","memory":"1Mi"}}}]},` +
		`"status":{"phase":"Running","conditions":[{"type":"Ready","status":"True"}]}}]}`
	fakeSyncFailedPDBList = `{"apiVersion":"policy/v1","kind":"PodDisruptionBudgetList","items":[{"metadata":{"name":"web","namespace":"default","generation":1},` +
		`"spec":{"minAvailable":1,"selector":{"matchLabels":{"app":"web"}}},` +
		`"status":{"observedGeneration":1,"currentHealthy":0,"desiredHealthy":0,"disruptionsAllowed":0,"expectedPods":1,` +
		`"conditions":[{"type":"DisruptionAllowed","status":"False","reason":"SyncFailed","message":"no scale subresource","lastTransitionTime":"2026-10-28T09:00:00Z"}]}}]}`
)

// cluster names a fake API served to the cases below.
type cluster int

const (
	clusterClean   cluster = iota // one node, no pods: no findings
	clusterBare                   // one node, a pod with no owner: BLOCKED, or RISK with --force
	clusterUnknown                // two nodes, a failed budget: one UNKNOWN finding only
)

func (c cluster) serve(t *testing.T) string {
	t.Helper()
	switch c {
	case clusterBare:
		return startFakeAPI(t, true)
	case clusterUnknown:
		return startFakeAPIWith(t, map[string]string{
			"nodes":                fakeTwoNodeList,
			"pods":                 fakeOwnedPodList,
			"poddisruptionbudgets": fakeSyncFailedPDBList,
		})
	default:
		return startFakeAPI(t, false)
	}
}

// TestBinaryExitCodesAndStdoutParity runs the built binary for each case twice,
// without and with --debug, and requires the same exit code and the same
// stdout. It protects two promises of docs/USAGE.md: the exit-code table, and
// that --debug writes to stderr only.
//
// Cases for a hang and for --request-timeout belong here once those flags exist
// (see also signals_test.go and closed_pipe_test.go for signals and closed pipes).
func TestBinaryExitCodesAndStdoutParity(t *testing.T) {
	bin := buildBinary(t)

	cases := []struct {
		name    string
		cluster cluster
		args    []string
		want    int
		// stdout is a substring the report must contain, so a case cannot
		// pass because the fixture stopped producing what it is named for.
		stdout string
		json   bool
		// early marks a run that ends before the cluster is read (help,
		// version, a bad flag): the logger does not exist yet, so --debug
		// writes no DEBUG line.
		early bool
	}{
		// 0: nothing at or above the threshold.
		{name: "clean, fail-on blocked", cluster: clusterClean, args: []string{"--nodes", "n1"}, want: 0, stdout: "No findings reported."},
		{name: "clean, fail-on risk", cluster: clusterClean, args: []string{"--nodes", "n1", "--fail-on", "risk"}, want: 0, stdout: "No findings reported."},
		{name: "clean, fail-on any", cluster: clusterClean, args: []string{"--nodes", "n1", "--fail-on", "any"}, want: 0, stdout: "No findings reported."},
		{name: "clean, json", cluster: clusterClean, args: []string{"--nodes", "n1", "--output", "json"}, want: 0, json: true},
		{name: "clean, all nodes", cluster: clusterClean, args: []string{"--nodes", "*"}, want: 0, stdout: "No findings reported."},

		// BLOCKED reaches every threshold.
		{name: "blocked, fail-on blocked", cluster: clusterBare, args: []string{"--nodes", "n1"}, want: 1, stdout: "BLOCKED: 1"},
		{name: "blocked, fail-on risk", cluster: clusterBare, args: []string{"--nodes", "n1", "--fail-on", "risk"}, want: 1, stdout: "BLOCKED: 1"},
		{name: "blocked, fail-on any", cluster: clusterBare, args: []string{"--nodes", "n1", "--fail-on", "any"}, want: 1, stdout: "BLOCKED: 1"},
		{name: "blocked, json", cluster: clusterBare, args: []string{"--nodes", "n1", "--output", "json"}, want: 1, json: true},
		{name: "blocked, verbose", cluster: clusterBare, args: []string{"--nodes", "n1", "--verbose"}, want: 1, stdout: "BLOCKED: 1"},
		{name: "blocked, all nodes", cluster: clusterBare, args: []string{"--nodes", "*"}, want: 1, stdout: "BLOCKED: 1"},

		// RISK (a bare pod with --force) trips risk and any, not blocked.
		{name: "risk, fail-on blocked", cluster: clusterBare, args: []string{"--nodes", "n1", "--force"}, want: 0, stdout: "RISK:    1"},
		{name: "risk, fail-on risk", cluster: clusterBare, args: []string{"--nodes", "n1", "--force", "--fail-on", "risk"}, want: 1, stdout: "RISK:    1"},
		{name: "risk, fail-on any", cluster: clusterBare, args: []string{"--nodes", "n1", "--force", "--fail-on", "any"}, want: 1, stdout: "RISK:    1"},
		{name: "risk, json", cluster: clusterBare, args: []string{"--nodes", "n1", "--force", "--fail-on", "risk", "--output", "json"}, want: 1, json: true},

		// UNKNOWN trips only any.
		{name: "unknown, fail-on blocked", cluster: clusterUnknown, args: []string{"--nodes", "n1"}, want: 0, stdout: "UNKNOWN: 1"},
		{name: "unknown, fail-on risk", cluster: clusterUnknown, args: []string{"--nodes", "n1", "--fail-on", "risk"}, want: 0, stdout: "UNKNOWN: 1"},
		{name: "unknown, fail-on any", cluster: clusterUnknown, args: []string{"--nodes", "n1", "--fail-on", "any"}, want: 1, stdout: "UNKNOWN: 1"},

		// Not a drain: 0 without touching the cluster.
		{name: "help", cluster: clusterClean, args: []string{"--help"}, want: 0, stdout: "--fail-on", early: true},
		{name: "version", cluster: clusterClean, args: []string{"--version"}, want: 0, stdout: "draincheck", early: true},

		// 2: draincheck could not complete. Nothing on stdout.
		{name: "node not in cluster", cluster: clusterClean, args: []string{"--nodes", "n9"}, want: 2},
		{name: "one of two nodes not in cluster", cluster: clusterClean, args: []string{"--nodes", "n1,n9"}, want: 2},
		{name: "missing --nodes", cluster: clusterClean, args: nil, want: 2, early: true},
		{name: "empty node name", cluster: clusterClean, args: []string{"--nodes", "n1,,n2"}, want: 2, early: true},
		{name: "wildcard with a name", cluster: clusterClean, args: []string{"--nodes", "*,n1"}, want: 2, early: true},
		{name: "bad --fail-on", cluster: clusterClean, args: []string{"--nodes", "n1", "--fail-on", "never"}, want: 2, early: true},
		{name: "bad --output", cluster: clusterClean, args: []string{"--nodes", "n1", "--output", "yaml"}, want: 2, early: true},
		{name: "bad --pod-selector", cluster: clusterClean, args: []string{"--nodes", "n1", "--pod-selector", "app in (web"}, want: 2, early: true},
		{name: "unknown flag", cluster: clusterClean, args: []string{"--nodes", "n1", "--no-such-flag"}, want: 2, early: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			kubeconfig := writeKubeconfig(t, tc.cluster.serve(t))
			base := append([]string{"--kubeconfig", kubeconfig}, tc.args...)

			code, stdout, stderr := run(t, bin, base...)
			if code != tc.want {
				t.Fatalf("exit code = %d, want %d\nstdout:\n%s\nstderr:\n%s", code, tc.want, stdout, stderr)
			}
			checkOutput(t, tc.want, tc.json, tc.stdout, stdout, stderr)

			dCode, dStdout, dStderr := run(t, bin, append(base, "--debug")...)
			if dCode != code {
				t.Errorf("with --debug exit code = %d, want %d\nstderr:\n%s", dCode, code, dStderr)
			}
			if maskTimestamp(dStdout) != maskTimestamp(stdout) {
				t.Errorf("stdout differs with --debug\nwithout:\n%s\nwith:\n%s", stdout, dStdout)
			}
			if got := strings.Contains(dStderr, "level=DEBUG"); got == tc.early {
				t.Errorf("DEBUG line on stderr = %t, want %t:\n%s", got, !tc.early, dStderr)
			}
		})
	}
}

// checkOutput holds what every case promises about its streams.
func checkOutput(t *testing.T, want int, isJSON bool, contains, stdout, stderr string) {
	t.Helper()
	if want == 2 {
		if stdout != "" {
			t.Errorf("exit 2 with stdout %q, want none", stdout)
		}
		if !strings.Contains(stderr, "error:") && !strings.Contains(stderr, "Usage:") {
			t.Errorf("exit 2 without an error message on stderr: %q", stderr)
		}
		return
	}
	if strings.Contains(stderr, "level=DEBUG") {
		t.Errorf("DEBUG lines on stderr without --debug:\n%s", stderr)
	}
	if contains != "" && !strings.Contains(stdout, contains) {
		t.Errorf("stdout does not contain %q:\n%s", contains, stdout)
	}
	if isJSON {
		var doc struct {
			SchemaVersion string `json:"schema_version"`
		}
		if err := json.Unmarshal([]byte(stdout), &doc); err != nil || doc.SchemaVersion != "v1" {
			t.Errorf("stdout is not a v1 JSON report (err %v):\n%s", err, stdout)
		}
	}
}

// TestBinaryExitCodesWhenTheClusterCannotBeRead covers the failures that need
// a server that is wrong rather than a flag that is: none behind the address,
// a missing kubeconfig, and an API that answers with an error.
func TestBinaryExitCodesWhenTheClusterCannotBeRead(t *testing.T) {
	bin := buildBinary(t)

	closed := httptest.NewServer(http.NotFoundHandler())
	closedURL := closed.URL
	closed.Close() // nothing listens here any more: connection refused

	denied := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"Forbidden","code":403,"message":"nodes is forbidden"}`))
	}))
	defer denied.Close()

	cases := []struct {
		name       string
		kubeconfig string
		stderr     string
	}{
		{"server unreachable", writeKubeconfig(t, closedURL), ""},
		{"server denies the list", writeKubeconfig(t, denied.URL), "forbidden"},
		{"kubeconfig missing", filepath.Join(t.TempDir(), "no-such-kubeconfig"), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"--kubeconfig", tc.kubeconfig, "--nodes", "n1"}
			code, stdout, stderr := run(t, bin, args...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, "error:") {
				t.Fatalf("exit %d, stdout %q, stderr %q; want exit 2, no stdout and an error", code, stdout, stderr)
			}
			if !strings.Contains(strings.ToLower(stderr), tc.stderr) {
				t.Errorf("stderr does not mention %q:\n%s", tc.stderr, stderr)
			}
			dCode, dStdout, dStderr := run(t, bin, append(args, "--debug")...)
			if dCode != 2 || dStdout != "" {
				t.Errorf("with --debug: exit %d, stdout %q; want exit 2, no stdout", dCode, dStdout)
			}
			if !strings.Contains(dStderr, "msg=finished exit_code=2") {
				t.Errorf("--debug did not log the exit code:\n%s", dStderr)
			}
		})
	}
}
