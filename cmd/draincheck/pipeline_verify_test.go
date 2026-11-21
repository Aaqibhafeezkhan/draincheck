package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/inventory"
)

// The JSON report carries the verify commands of each finding, with the names
// of that finding, and the run says in --debug how many it added. stdout is the
// same with and without --debug.
func TestExecuteWritesVerifyCommands(t *testing.T) {
	opts := options{Nodes: []string{"worker-1"}, Output: outputJSON, FailOn: failOnRisk}
	var plain, out, logs bytes.Buffer
	c := atFixedTime{inventory.NewClientsetCollector(labelledCluster("ssd"), nil)}
	if err := execute(context.Background(), opts, nil, c, &plain); err != nil && exitCodeOf(err) != exitFindings {
		t.Fatal(err)
	}
	if err := execute(context.Background(), opts, debugLog(&logs), c, &out); err != nil && exitCodeOf(err) != exitFindings {
		t.Fatal(err)
	}

	var doc struct {
		Findings []struct {
			RuleID string   `json:"rule_id"`
			Verify []string `json:"verify"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Findings) == 0 {
		t.Fatal("no findings")
	}
	for _, f := range doc.Findings {
		if len(f.Verify) == 0 {
			t.Errorf("%s: no verify commands in the report", f.RuleID)
		}
		for _, cmd := range f.Verify {
			if !strings.HasPrefix(cmd, "kubectl get ") && !strings.HasPrefix(cmd, "kubectl describe ") {
				t.Errorf("%s: %q is not a read-only command", f.RuleID, cmd)
			}
		}
	}
	if !strings.Contains(logs.String(), `msg="verify commands" findings=`) {
		t.Errorf("debug output lacks the verify commands line:\n%s", logs.String())
	}
	if plain.String() != out.String() {
		t.Errorf("--debug changed stdout")
	}
}
