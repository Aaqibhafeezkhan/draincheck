package analyzer

import (
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

// Every case in the catalog is written for a rule that exists, and has both
// texts, so a report never prints an empty Why or an empty remediation.
func TestDefaultCatalogCasesBelongToDefaultRulesAndAreComplete(t *testing.T) {
	ids := map[string]bool{}
	for _, r := range DefaultRules() {
		ids[r.ID()] = true
	}

	cat := DefaultCatalog()
	if len(cat) == 0 {
		t.Fatal("the catalog is empty")
	}
	for key, c := range cat {
		rule, _, ok := strings.Cut(key, "/")
		if !ok || !ids[rule] {
			t.Errorf("case %q is not of a default rule (key is <RULE>/<case>)", key)
		}
		if strings.TrimSpace(c.Why) == "" || strings.TrimSpace(c.Remediation) == "" {
			t.Errorf("case %q needs a Why and a Remediation: %+v", key, c)
		}
		if strings.Contains(c.Why, "\n") {
			t.Errorf("case %q: Why must be one sentence on one line: %q", key, c.Why)
		}
	}
}

// A rule without a case could not print any advice: every default rule has at
// least one case in the catalog.
func TestEveryDefaultRuleHasACase(t *testing.T) {
	cat := DefaultCatalog()
	for _, r := range DefaultRules() {
		found := false
		for key := range cat {
			if strings.HasPrefix(key, r.ID()+"/") {
				found = true
			}
		}
		if !found {
			t.Errorf("rule %s has no case in the default catalog", r.ID())
		}
	}
}

func TestDefaultCatalogHoldsTheCasesOfPodOwnership(t *testing.T) {
	cat := DefaultCatalog()
	for _, id := range []string{podOwnershipDaemonSet, podOwnershipStaticPod, podOwnershipNoController, podOwnershipNoControllerForced} {
		if _, ok := cat.Lookup(findings.Finding{RuleID: PodOwnershipID, Case: id}); !ok {
			t.Errorf("no case %s/%s in the catalog", PodOwnershipID, id)
		}
	}
}
