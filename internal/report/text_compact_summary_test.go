package report

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

// ruleFindings returns one finding per namespace entry; "" makes a
// cluster-scoped finding.
func ruleFindings(rule string, namespaces []string) []findings.Finding {
	fs := make([]findings.Finding, len(namespaces))
	for i, ns := range namespaces {
		kind := "Pod"
		if ns == "" {
			kind = "Node"
		}
		fs[i] = findings.Finding{
			RuleID:      rule,
			Severity:    findings.Risk,
			Resource:    findings.ResourceRef{Kind: kind, Namespace: ns, Name: fmt.Sprintf("r-%02d", i)},
			Description: "d",
			Remediation: "Fix it.",
		}
	}
	return fs
}

func TestCompactSummaryLine(t *testing.T) {
	tests := []struct {
		name       string
		namespaces []string
		want       string // rest of the rule header after "[SOME_RULE] "; "" means a plain header
	}{
		{"several namespaces", []string{"a", "a", "a", "b", "b", "c", "d"}, "7 findings in 4 namespaces, first 5 shown\n"},
		{"one namespace", []string{"a", "a", "a", "a", "a", "a"}, "6 findings in 1 namespace, first 5 shown\n"},
		{"no namespace at all", []string{"", "", "", "", "", ""}, "6 findings, first 5 shown\n"},
		{"cluster-scoped are not a namespace", []string{"a", "", "", "", "b", ""}, "6 findings in 2 namespaces, first 5 shown\n"},
		{"not collapsed", []string{"a", "b", "c", "d", "e"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(renderText(t, ruleFindings("SOME_RULE", tt.namespaces)))
			if tt.want == "" {
				if strings.Contains(got, "first 5 shown") || strings.Contains(got, "Summary:") {
					t.Errorf("uncollapsed rule printed collapse info in its header:\n%s", got)
				}
				return
			}
			// The totals are part of the rule header; no separate line follows it.
			header := "[SOME_RULE] " + tt.want
			if !strings.Contains(got, "\n"+header+"  Pod/") && !strings.Contains(got, "\n"+header+"  Node/") {
				t.Errorf("want header %q followed by the first finding, got:\n%s", header, got)
			}
			if !strings.Contains(got, fmt.Sprintf("... and %d more", len(tt.namespaces)-compactLimit)) {
				t.Errorf("'... and M more' missing or wrong:\n%s", got)
			}
		})
	}
}

func TestCompactSummaryIsIndependentOfInputOrder(t *testing.T) {
	fs := append(ruleFindings("A_RULE", []string{"a", "b", "c", "a", "b", "c", "d"}),
		ruleFindings("B_RULE", []string{"x", "x", "x", "x", "x", "x"})...)
	want := renderText(t, fs)
	for i := 0; i < 5; i++ {
		shuffled := slices.Clone(fs)
		slices.Reverse(shuffled)
		shuffled = append(shuffled[i:], shuffled[:i]...)
		if got := renderText(t, shuffled); string(got) != string(want) {
			t.Fatalf("output depends on input order:\n%s\nvs\n%s", got, want)
		}
	}
}

func TestVerbosePrintsNoSummaryLine(t *testing.T) {
	got := string(renderVerbose(t, ruleFindings("SOME_RULE", []string{"a", "b", "c", "d", "e", "f", "g"})))
	if strings.Contains(got, "findings in") || strings.Contains(got, "first 5 shown") {
		t.Errorf("verbose printed collapse info:\n%s", got)
	}
}

func TestCompactSummaryGolden(t *testing.T) {
	var fs []findings.Finding
	fs = append(fs, ruleFindings("ACROSS_NAMESPACES", []string{"a", "a", "a", "b", "b", "c", "d"})...)
	fs = append(fs, ruleFindings("ONE_NAMESPACE", []string{"a", "a", "a", "a", "a", "a"})...)
	fs = append(fs, ruleFindings("NO_NAMESPACE", []string{"", "", "", "", "", ""})...)

	got := renderText(t, fs)
	path := filepath.Join("..", "..", "testdata", "report", "text-compact-summary.golden.txt")
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
