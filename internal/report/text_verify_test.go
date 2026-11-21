package report

import (
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func verifyFinding(name string, sev findings.Severity, cmds ...string) findings.Finding {
	return findings.Finding{
		RuleID:      "R",
		Severity:    sev,
		Resource:    findings.ResourceRef{Kind: "Pod", Namespace: "shop", Name: name},
		Description: "Pod shop/" + name + " is a problem.",
		Verify:      cmds,
	}
}

// The compact layout prints the commands of a BLOCKED or UNKNOWN finding under
// its description, and none for a RISK, so the long list stays short.
func TestCompactPrintsVerifyOnlyForBlockedAndUnknown(t *testing.T) {
	fs := []findings.Finding{
		verifyFinding("blocked", findings.Blocked, "kubectl get pod -n shop blocked"),
		verifyFinding("unknown", findings.Unknown, "kubectl get pod -n shop unknown"),
		verifyFinding("risk", findings.Risk, "kubectl get pod -n shop risk"),
	}

	got := string(renderTextMode(t, fs, false))

	for _, want := range []string{"kubectl get pod -n shop blocked", "kubectl get pod -n shop unknown"} {
		if !strings.Contains(got, want) {
			t.Errorf("compact output lacks %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "kubectl get pod -n shop risk") {
		t.Errorf("compact output prints the commands of a RISK finding:\n%s", got)
	}
}

// The commands sit right under the description they confirm, one per line.
func TestCompactVerifyLayout(t *testing.T) {
	f := verifyFinding("blocked", findings.Blocked, "kubectl get pod -n shop blocked", "kubectl describe pod -n shop blocked")

	got := string(renderTextMode(t, []findings.Finding{f}, false))

	want := "  Pod/shop/blocked: Pod shop/blocked is a problem.\n" +
		"    Verify:\n" +
		"      kubectl get pod -n shop blocked\n" +
		"      kubectl describe pod -n shop blocked\n"
	if !strings.Contains(got, want) {
		t.Errorf("layout differs\n got:\n%s\nwant it to contain:\n%s", got, want)
	}
}

// --verbose prints the commands of every finding, whatever its severity.
func TestVerbosePrintsVerifyForEverySeverity(t *testing.T) {
	fs := []findings.Finding{
		verifyFinding("blocked", findings.Blocked, "kubectl get pod -n shop blocked"),
		verifyFinding("risk", findings.Risk, "kubectl get pod -n shop risk"),
	}

	got := string(renderTextMode(t, fs, true))

	for _, want := range []string{"kubectl get pod -n shop blocked", "kubectl get pod -n shop risk"} {
		if !strings.Contains(got, want) {
			t.Errorf("verbose output lacks %q:\n%s", want, got)
		}
	}
}

// A finding with no commands prints no Verify line.
func TestNoVerifyLineWithoutCommands(t *testing.T) {
	for _, verbose := range []bool{false, true} {
		got := string(renderTextMode(t, []findings.Finding{verifyFinding("x", findings.Blocked)}, verbose))
		if strings.Contains(got, "Verify") {
			t.Errorf("verbose=%v: a finding with no commands prints a Verify line:\n%s", verbose, got)
		}
	}
}
