package findings

import (
	"encoding/json"
	"testing"
)

func TestCaseKeyJoinsRuleAndCase(t *testing.T) {
	if got, want := CaseKey("POD_OWNERSHIP", "daemonset"), "POD_OWNERSHIP/daemonset"; got != want {
		t.Errorf("CaseKey = %q, want %q", got, want)
	}
}

func TestFindingJSONCarriesTheCase(t *testing.T) {
	b, err := json.Marshal(Finding{RuleID: "POD_OWNERSHIP", Case: "daemonset"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["case"] != "daemonset" {
		t.Errorf("case = %v in %s, want daemonset", got["case"], b)
	}
}

// The JSON report names the case and carries none of its text, so a finding
// has no remediation or why field.
func TestFindingJSONHasNoRemediationOrWhy(t *testing.T) {
	b, err := json.Marshal(Finding{RuleID: "POD_OWNERSHIP", Case: "daemonset", Description: "d"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"remediation", "why", "tip"} {
		if _, ok := got[key]; ok {
			t.Errorf("finding JSON has %q: %s", key, b)
		}
	}
}
