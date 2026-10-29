package report

import (
	"bytes"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func renderTextFlags(t *testing.T, fs []findings.Finding, flags DrainFlags) string {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteText(&buf, fs, snapshotTime, false, flags); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestWriteTextNamesTheMissingDrainFlags(t *testing.T) {
	got := renderTextFlags(t, sampleFindings(), DrainFlags{Missing: []string{"--ignore-daemonsets", "--force"}})

	for _, want := range []string{
		"=== KUBECTL DRAIN FLAGS ===\n",
		"kubectl drain will refuse without: --ignore-daemonsets --force\n",
		"derived from the BLOCKED findings",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
	summary := strings.Index(got, "=== FINDINGS SUMMARY ===")
	flags := strings.Index(got, "=== KUBECTL DRAIN FLAGS ===")
	blocked := strings.Index(got, "=== BLOCKED")
	if summary >= flags || flags >= blocked {
		t.Errorf("the flags section must sit between the summary and the findings (summary=%d flags=%d blocked=%d)", summary, flags, blocked)
	}
}

func TestWriteTextOmitsTheFlagsSectionWhenNothingIsMissing(t *testing.T) {
	got := renderTextFlags(t, sampleFindings(), DrainFlags{Missing: []string{}})
	if strings.Contains(got, "KUBECTL DRAIN FLAGS") {
		t.Errorf("section printed with nothing to say:\n%s", got)
	}
	if !bytes.Equal([]byte(got), renderText(t, sampleFindings())) {
		t.Error("an empty summary changed the report")
	}
}

func TestWriteTextSaysOnceThatEvictionIsDisabled(t *testing.T) {
	got := renderTextFlags(t, nil, DrainFlags{EvictionDisabled: true})
	line := "--disable-eviction is set: pods are deleted, not evicted, and PodDisruptionBudgets are not consulted.\n"
	if strings.Count(got, line) != 1 {
		t.Errorf("want the eviction line exactly once:\n%s", got)
	}
	if strings.Contains(got, "will refuse without") {
		t.Errorf("no flag is missing, but the report names one:\n%s", got)
	}
}

func TestWriteTextFlagsAndEvictionTogether(t *testing.T) {
	got := renderTextFlags(t, nil, DrainFlags{Missing: []string{"--force"}, EvictionDisabled: true})
	if strings.Count(got, "=== KUBECTL DRAIN FLAGS ===") != 1 {
		t.Errorf("want one section:\n%s", got)
	}
	for _, want := range []string{"will refuse without: --force", "--disable-eviction is set"} {
		if !strings.Contains(got, want) {
			t.Errorf("report lacks %q:\n%s", want, got)
		}
	}
}
