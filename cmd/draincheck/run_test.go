package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"
)

// secretToken is the bearer token of the test kubeconfig. It must never show
// up in diagnostic output.
const secretToken = "s3cr3t-token-value"

// fakeAPI serves one node, worker-1, with one pod that has no controller, and
// records the Authorization header of every request. It speaks TLS because
// client-go only reads the credentials of a kubeconfig for a TLS server.
type fakeAPI struct {
	*httptest.Server
	mu   sync.Mutex
	auth []string
}

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	lists := map[string]string{
		"/api/v1/nodes":                        `{"kind":"NodeList","apiVersion":"v1","items":[{"metadata":{"name":"worker-1"}}]}`,
		"/api/v1/pods":                         `{"kind":"PodList","apiVersion":"v1","items":[{"metadata":{"namespace":"default","name":"bare"},"spec":{"nodeName":"worker-1"}}]}`,
		"/apis/apps/v1/replicasets":            `{"kind":"ReplicaSetList","apiVersion":"apps/v1","items":[]}`,
		"/apis/policy/v1/poddisruptionbudgets": `{"kind":"PodDisruptionBudgetList","apiVersion":"policy/v1","items":[]}`,
		"/apis/apps/v1/deployments":            `{"kind":"DeploymentList","apiVersion":"apps/v1","items":[]}`,
		"/apis/apps/v1/statefulsets":           `{"kind":"StatefulSetList","apiVersion":"apps/v1","items":[]}`,
		"/apis/batch/v1/jobs":                  `{"kind":"JobList","apiVersion":"batch/v1","items":[]}`,
		"/apis/batch/v1/cronjobs":              `{"kind":"CronJobList","apiVersion":"batch/v1","items":[]}`,
	}
	api := &fakeAPI{}
	api.Server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		api.mu.Lock()
		api.auth = append(api.auth, r.Header.Get("Authorization"))
		api.mu.Unlock()
		body, ok := lists[r.URL.Path]
		if !ok || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, err := io.WriteString(w, body); err != nil {
			t.Errorf("fake API: write response: %v", err)
		}
	}))
	t.Cleanup(api.Close)
	return api
}

// kubeconfig returns the path of a kubeconfig with context "dev" for the server.
func (a *fakeAPI) kubeconfig(t *testing.T) string {
	t.Helper()
	return writeKubeconfig(t, fmt.Sprintf(`apiVersion: v1
kind: Config
current-context: dev
clusters:
- name: c
  cluster: {server: %q, insecure-skip-tls-verify: true}
users:
- name: u
  user: {token: %s}
contexts:
- name: dev
  context: {cluster: c, user: u}
`, a.URL, secretToken))
}

func (a *fakeAPI) authHeaders() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return slices.Clone(a.auth)
}

func TestRunWithDebugWritesDiagnosticsToStderrOnly(t *testing.T) {
	api := newFakeAPI(t)
	path := api.kubeconfig(t)
	opts := options{
		Nodes: []string{"worker-1"}, Kubeconfig: path, Context: "dev",
		Output: outputJSON, FailOn: failOnRisk, Debug: true,
	}
	var stdout, stderr bytes.Buffer

	err := runWith(context.Background(), opts, &stdout, &stderr)

	if exitCode(err) != exitFindings {
		t.Fatalf("exit code = %d (err %v), want %d", exitCode(err), err, exitFindings)
	}
	if !json.Valid(stdout.Bytes()) {
		t.Errorf("stdout is not the JSON report:\n%s", stdout.String())
	}
	wantInOrder(t, stderr.String(),
		`msg=settings kubeconfig=[`+path+`] context=dev nodes=[worker-1] all_nodes=false output=json verbose=false fail_on=risk`,
		`msg=list resource=nodes count=1`,
		`msg=list resource=pods count=1`,
		`msg="resolve target nodes"`,
		`msg="rule done" rule=POD_OWNERSHIP findings=1`,
		`msg=summary blocked=0 risk=1 unknown=0 fail_on=risk`,
		`msg="write report"`,
		`msg=finished exit_code=1`,
	)

	if !slices.Contains(api.authHeaders(), "Bearer "+secretToken) {
		t.Fatalf("the fake API never saw the token, so the test proves nothing: %q", api.authHeaders())
	}
	for name, out := range map[string]string{"stderr": stderr.String(), "stdout": stdout.String()} {
		if bytes.Contains([]byte(out), []byte(secretToken)) {
			t.Errorf("%s contains the kubeconfig token:\n%s", name, out)
		}
	}
}

func TestRunWithoutDebugWritesNothingToStderr(t *testing.T) {
	api := newFakeAPI(t)
	opts := options{Nodes: []string{"worker-1"}, Kubeconfig: api.kubeconfig(t), Output: outputJSON, FailOn: failOnBlocked}
	var stdout, stderr bytes.Buffer

	if err := runWith(context.Background(), opts, &stdout, &stderr); err != nil {
		t.Fatalf("runWith() error = %v", err)
	}

	if stderr.Len() != 0 {
		t.Errorf("stderr is not empty without --debug:\n%s", stderr.String())
	}
	if !json.Valid(stdout.Bytes()) {
		t.Errorf("stdout is not the JSON report:\n%s", stdout.String())
	}
}

func TestRunWithDebugLogsTheExitCode(t *testing.T) {
	api := newFakeAPI(t)
	tests := []struct {
		name     string
		nodes    []string
		wantCode int
	}{
		{"clean run", []string{"worker-1"}, 0},
		{"unknown node", []string{"ghost"}, exitFailure},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := options{Nodes: tt.nodes, Kubeconfig: api.kubeconfig(t), Output: outputJSON, FailOn: failOnBlocked, Debug: true}
			var stdout, stderr bytes.Buffer

			err := runWith(context.Background(), opts, &stdout, &stderr)

			if got := exitCode(err); got != tt.wantCode {
				t.Fatalf("exit code = %d (err %v), want %d", got, err, tt.wantCode)
			}
			wantInOrder(t, stderr.String(), fmt.Sprintf("msg=finished exit_code=%d", tt.wantCode))
		})
	}
}

func TestRunWithDebugLogsAMissingKubeconfig(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Kubeconfig: "/nonexistent/kubeconfig", Output: outputJSON, FailOn: failOnBlocked, Debug: true}
	var stdout, stderr bytes.Buffer

	err := runWith(context.Background(), opts, &stdout, &stderr)

	if exitCode(err) != exitFailure {
		t.Fatalf("exit code = %d (err %v), want %d", exitCode(err), err, exitFailure)
	}
	wantInOrder(t, stderr.String(), `kubeconfig=[/nonexistent/kubeconfig]`, `msg=finished exit_code=2`)
	if stdout.Len() != 0 {
		t.Errorf("wrote output on error: %s", stdout.String())
	}
}
