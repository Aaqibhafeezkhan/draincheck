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

func TestCaseJSON(t *testing.T) {
	b, err := json.Marshal(Case{Why: "Why text.", Remediation: "Fix it."})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"why":"Why text.","remediation":"Fix it."}`
	if string(b) != want {
		t.Errorf("json mismatch\n got: %s\nwant: %s", b, want)
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
