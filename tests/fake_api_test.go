//go:build !windows

package tests

import (
	"net/http"
	"net/http/httptest"
	"path"
	"testing"
)

const (
	fakeNodeList  = `{"apiVersion":"v1","kind":"NodeList","items":[{"metadata":{"name":"n1"},"status":{"conditions":[{"type":"Ready","status":"True"}]}}]}`
	fakePodList   = `{"apiVersion":"v1","kind":"PodList","items":[{"metadata":{"name":"bare","namespace":"default"},"spec":{"nodeName":"n1","containers":[{"name":"c","resources":{"requests":{"cpu":"1m"}}}]},"status":{"phase":"Running"}}]}`
	fakeEmptyList = `{"apiVersion":"v1","kind":"List","items":[]}`
)

// startFakeAPI serves list responses for one node, n1. With barePod it also
// serves a pod without an owner, so a drain of n1 has a RISK finding (POD_OWNERSHIP)
// (exit 1 with --fail-on risk); without it the run is clean (exit 0). Every other list is empty.
func startFakeAPI(t *testing.T, barePod bool) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := fakeEmptyList
		switch path.Base(r.URL.Path) {
		case "nodes":
			body = fakeNodeList
		case "pods":
			if barePod {
				body = fakePodList
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv.URL
}
