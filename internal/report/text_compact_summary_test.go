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
		want       string // "" means no summary line
	}{
		{"several namespaces", []string{"a", "a", "a", "b", "b", "c", "d"}, "  7 findings in 4 namespaces\n"},
		{"one namespace", []string{"a", "a", "a", "a", "a", "a"}, "  6 findings in 1 namespace\n"},
		{"no namespace at all", []string{"", "", "", "", "", ""}, "  6 findings\n"},
		{"cluster-scoped are not a namespace", []string{"a", "", "", "", "b", ""}, "  6 findings in 2 namespaces\n"},
		{"not collapsed", []string{"a", "b", "c", "d", "e"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(renderText(t, ruleFindings("SOME_RULE", tt.namespaces)))
			if tt.want == "" {
				if strings.Contains(got, " findings in ") || strings.Contains(got, "  5 findings\n") {
					t.Errorf("uncollapsed rule printed a summary line:\n%s", got)
				}
				return
			}
			// The line sits between the rule header and the first example.
			header := fmt.Sprintf("[SOME_RULE] %d findings\n", len(tt.namespaces))
			i := strings.Index(got, header+tt.want+"  Pod/")
			j := strings.Index(got, header+tt.want+"  Node/")
			if i < 0 && j < 0 {
				t.Errorf("want %q right after the rule header, got:\n%s", tt.want, got)
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
	if strings.Contains(got, "findings in") {
		t.Errorf("verbose printed a summary line:\n%s", got)
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
