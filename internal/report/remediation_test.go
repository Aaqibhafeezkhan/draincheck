package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func renderWithCatalog(t *testing.T, fs []findings.Finding, cat findings.Catalog, verbose bool) string {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteText(&buf, fs, snapshotTime, verbose, DrainFlags{}, cat); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestRemediationOfPrefersTheCatalog(t *testing.T) {
	cat := findings.Catalog{"R/c": {Why: "w", Remediation: "From the catalog."}}
	f := findings.Finding{RuleID: "R", Case: "c", Remediation: "From the finding."}

	if got := remediationOf(cat, f); got != "From the catalog." {
		t.Errorf("remediationOf = %q, want the catalog text", got)
	}
}

func TestRemediationOfFallsBackToTheFinding(t *testing.T) {
	cat := findings.Catalog{"R/c": {Remediation: "From the catalog."}}

	for name, f := range map[string]findings.Finding{
		"no case":      {RuleID: "R", Remediation: "From the finding."},
		"unknown case": {RuleID: "R", Case: "other", Remediation: "From the finding."},
	} {
		if got := remediationOf(cat, f); got != "From the finding." {
			t.Errorf("%s: remediationOf = %q, want the finding text", name, got)
		}
	}
	if got := remediationOf(nil, findings.Finding{Remediation: "x"}); got != "x" {
		t.Errorf("nil catalog: remediationOf = %q, want x", got)
	}
}

func TestCompactPrintsTheCatalogRemediationOncePerCase(t *testing.T) {
	cat := findings.Catalog{
		"POD_OWNERSHIP/daemonset":  {Remediation: "Drain with --ignore-daemonsets."},
		"POD_OWNERSHIP/static_pod": {Remediation: "Plan for the static pod outage."},
	}
	res := func(n string) findings.ResourceRef { return findings.ResourceRef{Kind: "Pod", Namespace: "a", Name: n} }
	fs := []findings.Finding{
		{RuleID: "POD_OWNERSHIP", Severity: findings.Risk, Resource: res("ds-1"), Case: "daemonset", Description: "d1"},
		{RuleID: "POD_OWNERSHIP", Severity: findings.Risk, Resource: res("ds-2"), Case: "daemonset", Description: "d2"},
		{RuleID: "POD_OWNERSHIP", Severity: findings.Risk, Resource: res("st-1"), Case: "static_pod", Description: "d3"},
	}

	got := renderWithCatalog(t, fs, cat, false)

	for _, want := range []string{
		"\n  => Remediation (Pod/a/ds-1, Pod/a/ds-2): Drain with --ignore-daemonsets.\n",
		"\n  => Remediation (Pod/a/st-1): Plan for the static pod outage.\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
}

func TestVerbosePrintsTheCatalogRemediationPerFinding(t *testing.T) {
	cat := findings.Catalog{"R/c": {Remediation: "From the catalog."}}
	fs := []findings.Finding{
		{RuleID: "R", Severity: findings.Risk, Resource: findings.ResourceRef{Kind: "Pod", Name: "a"}, Case: "c", Description: "d"},
		{RuleID: "R", Severity: findings.Risk, Resource: findings.ResourceRef{Kind: "Pod", Name: "b"}, Case: "c", Description: "d"},
	}

	got := renderWithCatalog(t, fs, cat, true)

	if n := strings.Count(got, "  Remediation: From the catalog.\n"); n != 2 {
		t.Errorf("verbose printed the catalog remediation %d times, want 2:\n%s", n, got)
	}
}
