package report

import (
	"bytes"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func renderText(t *testing.T, fs []findings.Finding) []byte {
	t.Helper()
	return renderTextMode(t, fs, false)
}

func renderVerbose(t *testing.T, fs []findings.Finding) []byte {
	t.Helper()
	return renderTextMode(t, fs, true)
}

func renderTextMode(t *testing.T, fs []findings.Finding, verbose bool) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteText(&buf, fs, snapshotTime, verbose); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestWriteTextGroupsBySeverityBlockedFirst(t *testing.T) {
	got := string(renderText(t, sampleFindings()))

	blocked := strings.Index(got, "=== BLOCKED (2) ===")
	risk := strings.Index(got, "=== RISK (2) ===")
	unknown := strings.Index(got, "=== UNKNOWN (1) ===")
	if blocked < 0 || risk < 0 || unknown < 0 {
		t.Fatalf("missing a severity section:\n%s", got)
	}
	if blocked >= risk || risk >= unknown {
		t.Errorf("sections out of order (BLOCKED=%d RISK=%d UNKNOWN=%d):\n%s", blocked, risk, unknown, got)
	}
}

func TestWriteTextSummaryCountsEverySeverity(t *testing.T) {
	got := string(renderText(t, sampleFindings()))

	for _, line := range []string{"  BLOCKED: 2\n", "  RISK:    2\n", "  UNKNOWN: 1\n"} {
		if !strings.Contains(got, line) {
			t.Errorf("summary missing %q:\n%s", line, got)
		}
	}
}

func TestWriteTextOmitsSectionsWithoutFindings(t *testing.T) {
	fs := []findings.Finding{{
		RuleID:      "POD_OWNERSHIP",
		Severity:    findings.Risk,
		Resource:    findings.ResourceRef{Kind: "Pod", Namespace: "ns", Name: "p"},
		Description: "d",
		Remediation: "r",
	}}

	got := string(renderText(t, fs))

	if strings.Contains(got, "=== BLOCKED") || strings.Contains(got, "=== UNKNOWN") {
		t.Errorf("empty severity sections must be omitted:\n%s", got)
	}
	if !strings.Contains(got, "  BLOCKED: 0\n") {
		t.Errorf("summary must still list zero counts:\n%s", got)
	}
}

func TestWriteTextNoFindingsIsNotAGuarantee(t *testing.T) {
	got := string(renderText(t, nil))

	if !strings.Contains(got, "No findings reported.") {
		t.Errorf("missing no-findings line:\n%s", got)
	}
	if strings.Contains(strings.ToLower(got), "safe") {
		t.Errorf("output must never say SAFE:\n%s", got)
	}
}

func TestWriteTextIsOrderIndependent(t *testing.T) {
	fs := sampleFindings()
	want := renderText(t, fs)

	reversed := slices.Clone(fs)
	slices.Reverse(reversed)
	if got := renderText(t, reversed); !bytes.Equal(got, want) {
		t.Errorf("output depends on input order\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteTextDoesNotMutateInput(t *testing.T) {
	fs := sampleFindings()
	before := slices.Clone(fs)

	renderText(t, fs)

	for i := range fs {
		if fs[i].Resource != before[i].Resource || fs[i].RuleID != before[i].RuleID {
			t.Fatalf("input slice was reordered at index %d", i)
		}
	}
}

func TestWriteTextEndsWithNewline(t *testing.T) {
	for name, in := range map[string][]findings.Finding{"none": nil, "some": sampleFindings()} {
		t.Run(name, func(t *testing.T) {
			if got := renderText(t, in); !bytes.HasSuffix(got, []byte("\n")) {
				t.Errorf("output must end with a newline: %q", got)
			}
		})
	}
}

func TestWriteTextReturnsWriterError(t *testing.T) {
	want := errors.New("disk full")

	err := WriteText(failingWriter{err: want}, sampleFindings(), snapshotTime, false)

	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}

func TestWriteTextShowsSnapshotTimeAndDisclaimer(t *testing.T) {
	for name, in := range map[string][]findings.Finding{"none": nil, "some": sampleFindings()} {
		t.Run(name, func(t *testing.T) {
			got := string(renderText(t, in))

			if !strings.Contains(got, "Timestamp: 2026-10-10T21:00:00Z\n") {
				t.Errorf("missing snapshot timestamp:\n%s", got)
			}
			if !strings.HasSuffix(got, Disclaimer(snapshotTime)+"\n") {
				t.Errorf("disclaimer must be the last line:\n%s", got)
			}
		})
	}
}
