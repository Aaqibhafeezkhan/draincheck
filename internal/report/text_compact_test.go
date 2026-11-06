package report

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

// manyFindings returns n findings of one rule that share one case, whose
// remediation is in testCatalog.
func manyFindings(n int) []findings.Finding {
	fs := make([]findings.Finding, n)
	for i := range fs {
		fs[i] = findings.Finding{
			RuleID:      "MISSING_RESOURCE_REQUESTS",
			Severity:    findings.Risk,
			Resource:    findings.ResourceRef{Kind: "Pod", Namespace: "shop", Name: fmt.Sprintf("web-%02d", i)},
			Case:        "missing_requests",
			Description: fmt.Sprintf("Pod shop/web-%02d has containers without resource requests.", i),
		}
	}
	return fs
}

func TestWriteTextCompactGolden(t *testing.T) {
	got := renderText(t, sampleFindings())

	path := filepath.Join("..", "..", "testdata", "report", "text-compact.golden.txt")
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s (rerun with -update if the change is deliberate)\n got:\n%s\nwant:\n%s", path, got, want)
	}
}

func TestCompactPrintsRemediationOnceVerbosePrintsPerFinding(t *testing.T) {
	fs := manyFindings(10)
	const remediation = "Set CPU and memory requests."

	if got := strings.Count(string(renderText(t, fs)), remediation); got != 1 {
		t.Errorf("compact: remediation printed %d times, want 1", got)
	}
	if got := strings.Count(string(renderVerbose(t, fs)), remediation); got != 10 {
		t.Errorf("verbose: remediation printed %d times, want 10", got)
	}
}

func TestCompactSetsTheAdviceApartFromFindings(t *testing.T) {
	got := string(renderText(t, manyFindings(3)))

	if !strings.Contains(got, "\n\n  => Why: Rescheduled pods may not fit where they land.\n  => Remediation: Set CPU and memory requests.\n") {
		t.Errorf("the advice must follow a blank line, with the remediation starting with =>:\n%s", got)
	}
	verbose := string(renderVerbose(t, manyFindings(3)))
	if strings.Contains(verbose, "=>") {
		t.Errorf("verbose output has no => marker:\n%s", verbose)
	}
}

func TestCompactGroupsFindingsByRuleWithCount(t *testing.T) {
	got := string(renderText(t, manyFindings(3)))

	if !strings.Contains(got, "[MISSING_RESOURCE_REQUESTS] 3 findings\n") {
		t.Errorf("missing rule header with count:\n%s", got)
	}
	if strings.Count(got, "[MISSING_RESOURCE_REQUESTS]") != 1 {
		t.Errorf("rule header must appear once:\n%s", got)
	}
}

func TestCompactSingularHeader(t *testing.T) {
	got := string(renderText(t, manyFindings(1)))

	if !strings.Contains(got, "[MISSING_RESOURCE_REQUESTS] 1 finding\n") {
		t.Errorf("want singular header:\n%s", got)
	}
}

func TestCompactCollapsesLongListsAndVerbosePrintsAll(t *testing.T) {
	fs := manyFindings(compactLimit + 5)

	compact := string(renderText(t, fs))
	if !strings.Contains(compact, "  ... and 5 more\n") {
		t.Errorf("compact output must end the list with the right count:\n%s", compact)
	}
	if strings.Contains(compact, fmt.Sprintf("web-%02d", compactLimit)) {
		t.Errorf("compact output printed a collapsed resource:\n%s", compact)
	}
	if !strings.Contains(compact, "--verbose") {
		t.Errorf("collapsed output must point at --verbose:\n%s", compact)
	}

	verbose := string(renderVerbose(t, fs))
	for _, f := range fs {
		if !strings.Contains(verbose, f.Resource.String()) {
			t.Errorf("verbose output is missing %s", f.Resource)
		}
	}
	if strings.Contains(verbose, "... and ") {
		t.Errorf("verbose output must not collapse:\n%s", verbose)
	}
}

func TestCompactDoesNotCollapseAtTheLimit(t *testing.T) {
	got := string(renderText(t, manyFindings(compactLimit)))

	if strings.Contains(got, "... and ") || strings.Contains(got, "--verbose") {
		t.Errorf("a list at the limit must not be collapsed:\n%s", got)
	}
}

func TestCompactKeepsSeverityOrderAndSummary(t *testing.T) {
	got := string(renderText(t, sampleFindings()))

	blocked := strings.Index(got, "=== BLOCKED (2) ===")
	risk := strings.Index(got, "=== RISK (2) ===")
	if blocked < 0 || risk < 0 || blocked >= risk {
		t.Errorf("severity sections out of order:\n%s", got)
	}
	if !strings.Contains(got, "  BLOCKED: 2\n") {
		t.Errorf("summary missing:\n%s", got)
	}
}

func TestCompactNeverLosesAFinding(t *testing.T) {
	fs := manyFindings(compactLimit + 3)

	// Every finding is either listed or counted in the collapsed tail.
	got := string(renderText(t, fs))
	listed := 0
	for _, f := range fs {
		if strings.Contains(got, f.Resource.String()) {
			listed++
		}
	}
	if listed != compactLimit {
		t.Errorf("listed %d resources, want %d plus a count for the rest:\n%s", listed, compactLimit, got)
	}
	if !strings.Contains(got, "... and 3 more") {
		t.Errorf("missing collapsed count:\n%s", got)
	}
}
