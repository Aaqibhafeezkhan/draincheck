package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/inventory"
)

// daemonSetCluster (pipeline_test.go) has a DaemonSet pod and a bare pod on
// worker-1, so without flags kubectl drain refuses both.
func TestExecuteLogsTheDrainFlagsTheRunStillNeeds(t *testing.T) {
	cases := []struct {
		name string
		opts options
		want string
	}{
		{"no flags", options{}, `msg="drain flags" missing="[--ignore-daemonsets --force]" eviction_disabled=false`},
		{"--ignore-daemonsets given", options{IgnoreDaemonSets: true}, `msg="drain flags" missing=[--force] eviction_disabled=false`},
		{"both given", options{IgnoreDaemonSets: true, Force: true}, `msg="drain flags" missing=[] eviction_disabled=false`},
		{"eviction disabled", options{IgnoreDaemonSets: true, Force: true, DisableEviction: true}, `msg="drain flags" missing=[] eviction_disabled=true`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			var out, logs bytes.Buffer
			opts := tt.opts
			opts.Nodes, opts.Output, opts.FailOn = []string{"worker-1"}, outputJSON, failOnBlocked
			_ = execute(context.Background(), opts, debugLog(&logs), inventory.NewClientsetCollector(daemonSetCluster(), nil), &out)
			wantInOrder(t, logs.String(), `msg=summary`, tt.want, `msg="write report"`)
		})
	}
}

func TestExecuteReportNamesTheDrainFlagsInTextAndJSON(t *testing.T) {
	var text bytes.Buffer
	opts := options{Nodes: []string{"worker-1"}, Output: outputText, FailOn: failOnBlocked}
	_ = execute(context.Background(), opts, nil, inventory.NewClientsetCollector(daemonSetCluster(), nil), &text)
	if !strings.Contains(text.String(), "kubectl drain will refuse without: --ignore-daemonsets --force\n") {
		t.Errorf("text report lacks the flags line:\n%s", text.String())
	}

	var js bytes.Buffer
	opts.Output = outputJSON
	_ = execute(context.Background(), opts, nil, inventory.NewClientsetCollector(daemonSetCluster(), nil), &js)
	var doc struct {
		DrainFlags struct {
			Missing []string `json:"missing"`
		} `json:"drain_flags"`
	}
	if err := json.Unmarshal(js.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if strings.Join(doc.DrainFlags.Missing, " ") != "--ignore-daemonsets --force" {
		t.Errorf("drain_flags.missing = %v", doc.DrainFlags.Missing)
	}
}

// A flag the operator passed is never named again.
func TestExecuteNeverNamesAFlagThatWasPassed(t *testing.T) {
	var out bytes.Buffer
	opts := options{Nodes: []string{"worker-1"}, Output: outputText, FailOn: failOnBlocked, IgnoreDaemonSets: true, Force: true}
	_ = execute(context.Background(), opts, nil, inventory.NewClientsetCollector(daemonSetCluster(), nil), &out)
	if strings.Contains(out.String(), "KUBECTL DRAIN FLAGS") {
		t.Errorf("report names flags although both were passed:\n%s", out.String())
	}
}

// --debug adds lines to stderr and nothing to stdout, also with the flags line.
func TestExecuteDebugDoesNotChangeStdoutWithDrainFlags(t *testing.T) {
	for _, format := range []string{outputText, outputJSON} {
		opts := options{Nodes: []string{"worker-1"}, Output: format, FailOn: failOnBlocked}
		c := atFixedTime{inventory.NewClientsetCollector(daemonSetCluster(), nil)}
		var plain, debug, logs bytes.Buffer
		_ = execute(context.Background(), opts, nil, c, &plain)
		_ = execute(context.Background(), opts, debugLog(&logs), c, &debug)
		if !bytes.Equal(plain.Bytes(), debug.Bytes()) {
			t.Errorf("%s: --debug changed stdout", format)
		}
	}
}
