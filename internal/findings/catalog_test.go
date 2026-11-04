package findings

import (
	"encoding/json"
	"testing"
)

func TestCatalogLookupFindsTheCaseOfAFinding(t *testing.T) {
	c := Catalog{
		"POD_OWNERSHIP/daemonset":  {Why: "w1", Remediation: "r1"},
		"POD_OWNERSHIP/static_pod": {Why: "w2", Remediation: "r2"},
	}

	got, ok := c.Lookup(Finding{RuleID: "POD_OWNERSHIP", Case: "static_pod"})
	if !ok || got.Why != "w2" || got.Remediation != "r2" {
		t.Errorf("Lookup = %+v, %v, want the static_pod case", got, ok)
	}
}

func TestCatalogLookupReportsAMissingCase(t *testing.T) {
	c := Catalog{"POD_OWNERSHIP/daemonset": {Why: "w", Remediation: "r"}}

	for name, f := range map[string]Finding{
		"no case on the finding": {RuleID: "POD_OWNERSHIP"},
		"unknown case":           {RuleID: "POD_OWNERSHIP", Case: "other"},
		"case of another rule":   {RuleID: "POD_LOCAL_STORAGE", Case: "daemonset"},
	} {
		if _, ok := c.Lookup(f); ok {
			t.Errorf("%s: Lookup found a case", name)
		}
	}
}

func TestCatalogLookupOnNilCatalog(t *testing.T) {
	var c Catalog
	if _, ok := c.Lookup(Finding{RuleID: "R", Case: "c"}); ok {
		t.Error("a nil catalog must find nothing")
	}
}

func TestCatalogJSONIsSortedByKey(t *testing.T) {
	c := Catalog{"B/x": {Why: "b"}, "A/y": {Why: "a"}}
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"A/y":{"why":"a","remediation":""},"B/x":{"why":"b","remediation":""}}`
	if string(b) != want {
		t.Errorf("json mismatch\n got: %s\nwant: %s", b, want)
	}
}
