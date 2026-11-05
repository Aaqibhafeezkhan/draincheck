package report

import (
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func TestWhyOfComesOnlyFromTheCatalog(t *testing.T) {
	cat := findings.Catalog{"R/c": {Why: "Because.", Remediation: "Fix."}}

	if got := whyOf(cat, findings.Finding{RuleID: "R", Case: "c"}); got != "Because." {
		t.Errorf("whyOf = %q, want the catalog text", got)
	}
	for name, f := range map[string]findings.Finding{
		"no case":      {RuleID: "R"},
		"unknown case": {RuleID: "R", Case: "other"},
	} {
		if got := whyOf(cat, f); got != "" {
			t.Errorf("%s: whyOf = %q, want empty", name, got)
		}
	}
	if got := whyOf(nil, findings.Finding{RuleID: "R", Case: "c"}); got != "" {
		t.Errorf("nil catalog: whyOf = %q, want empty", got)
	}
}

func caseFinding(rule, caseID, name string) findings.Finding {
	return findings.Finding{
		RuleID: rule, Severity: findings.Risk, Case: caseID, Description: "facts of " + name,
		Resource: findings.ResourceRef{Kind: "Pod", Namespace: "a", Name: name},
	}
}

func TestCompactPrintsTheWhyOncePerCaseBeforeTheRemediation(t *testing.T) {
	cat := findings.Catalog{"R/c": {Why: "Because.", Remediation: "Fix."}}
	fs := []findings.Finding{caseFinding("R", "c", "p1"), caseFinding("R", "c", "p2"), caseFinding("R", "c", "p3")}

	got := renderWithCatalog(t, fs, cat, false)

	if n := strings.Count(got, "Because."); n != 1 {
		t.Errorf("Why printed %d times, want 1:\n%s", n, got)
	}
	if !strings.Contains(got, "\n\n  => Why: Because.\n  => Remediation: Fix.\n") {
		t.Errorf("want the Why right above the remediation after a blank line:\n%s", got)
	}
}

func TestCompactNamesTheResourcesOfEachCaseWhenARuleHasSeveral(t *testing.T) {
	cat := findings.Catalog{
		"R/a": {Why: "Why a.", Remediation: "Fix a."},
		"R/b": {Why: "Why b.", Remediation: "Fix b."},
	}
	fs := []findings.Finding{caseFinding("R", "a", "p1"), caseFinding("R", "a", "p2"), caseFinding("R", "b", "p3")}

	got := renderWithCatalog(t, fs, cat, false)

	for _, want := range []string{
		"\n  => Why (Pod/a/p1, Pod/a/p2): Why a.\n  => Remediation: Fix a.\n",
		"\n  => Why (Pod/a/p3): Why b.\n  => Remediation: Fix b.\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

// A finding whose case the catalog does not hold has no advice, and the report
// does not print an empty Why or an empty remediation for it.
func TestFindingWithoutACaseHasNoAdvice(t *testing.T) {
	fs := []findings.Finding{{RuleID: "R", Severity: findings.Risk, Description: "facts", Resource: findings.ResourceRef{Kind: "Pod", Name: "p"}}}

	for name, got := range map[string]string{"compact": string(renderText(t, fs)), "verbose": string(renderVerbose(t, fs))} {
		if strings.Contains(got, "Why") || strings.Contains(got, "Remediation") {
			t.Errorf("%s: a finding without a case has no advice:\n%s", name, got)
		}
		if !strings.Contains(got, "facts") {
			t.Errorf("%s: the finding itself must still be printed:\n%s", name, got)
		}
	}
}

func TestVerbosePrintsTheWhyForEveryFindingWithACase(t *testing.T) {
	cat := findings.Catalog{"R/c": {Why: "Because.", Remediation: "Fix."}}
	fs := []findings.Finding{caseFinding("R", "c", "p1"), caseFinding("R", "c", "p2")}

	got := renderWithCatalog(t, fs, cat, true)

	if n := strings.Count(got, "  Why: Because.\n  Remediation: Fix.\n"); n != 2 {
		t.Errorf("verbose printed Why above the remediation %d times, want 2:\n%s", n, got)
	}
}
