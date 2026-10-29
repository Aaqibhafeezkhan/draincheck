package report

import (
	"slices"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func ownershipFinding(sev findings.Severity, cat findings.OwnershipCategory) findings.Finding {
	return findings.Finding{
		RuleID:   "POD_OWNERSHIP",
		Severity: sev,
		Evidence: findings.OwnershipEvidence{Category: cat},
	}
}

func localStorageFinding(sev findings.Severity, deleteEmptyDir bool) findings.Finding {
	return findings.Finding{
		RuleID:   "POD_LOCAL_STORAGE",
		Severity: sev,
		Evidence: findings.LocalStorageEvidence{DeleteEmptyDirData: deleteEmptyDir},
	}
}

func TestSummarizeDrainFlags(t *testing.T) {
	tests := []struct {
		name string
		fs   []findings.Finding
		want []string
	}{
		{"no findings", nil, []string{}},
		{"blocked DaemonSet", []findings.Finding{ownershipFinding(findings.Blocked, findings.OwnershipDaemonSet)}, []string{"--ignore-daemonsets"}},
		{"blocked unmanaged pod", []findings.Finding{ownershipFinding(findings.Blocked, findings.OwnershipUnmanaged)}, []string{"--force"}},
		{"blocked emptyDir", []findings.Finding{localStorageFinding(findings.Blocked, false)}, []string{"--delete-emptydir-data"}},
		{"static pod is a risk, no flag lifts it", []findings.Finding{ownershipFinding(findings.Risk, findings.OwnershipStaticPod)}, []string{}},
		{"unmanaged pod with --force is a risk", []findings.Finding{ownershipFinding(findings.Risk, findings.OwnershipUnmanaged)}, []string{}},
		{"emptyDir with the flag is a risk", []findings.Finding{localStorageFinding(findings.Risk, true)}, []string{}},
		{"blocked finding of another rule names no flag", []findings.Finding{{RuleID: "PDB_DISRUPTION_BLOCKED", Severity: findings.Blocked}}, []string{}},
		{"unknown finding names no flag", []findings.Finding{ownershipFinding(findings.Unknown, findings.OwnershipDaemonSet)}, []string{}},
		{"finding without evidence names no flag", []findings.Finding{{RuleID: "POD_OWNERSHIP", Severity: findings.Blocked}}, []string{}},
		{
			"all three, once each, in a fixed order",
			[]findings.Finding{
				localStorageFinding(findings.Blocked, false),
				ownershipFinding(findings.Blocked, findings.OwnershipUnmanaged),
				ownershipFinding(findings.Blocked, findings.OwnershipDaemonSet),
				ownershipFinding(findings.Blocked, findings.OwnershipDaemonSet),
				localStorageFinding(findings.Blocked, false),
			},
			[]string{"--ignore-daemonsets", "--delete-emptydir-data", "--force"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SummarizeDrainFlags(tt.fs, false)
			if got.Missing == nil || !slices.Equal(got.Missing, tt.want) {
				t.Errorf("Missing = %#v, want %#v (non-nil)", got.Missing, tt.want)
			}
			if got.EvictionDisabled {
				t.Error("EvictionDisabled = true, want false")
			}
		})
	}
}

func TestSummarizeDrainFlagsEvictionDisabled(t *testing.T) {
	if got := SummarizeDrainFlags(nil, true); !got.EvictionDisabled {
		t.Error("EvictionDisabled = false, want true")
	}
}

func TestSummarizeDrainFlagsIgnoresInputOrder(t *testing.T) {
	a := []findings.Finding{ownershipFinding(findings.Blocked, findings.OwnershipUnmanaged), ownershipFinding(findings.Blocked, findings.OwnershipDaemonSet)}
	b := []findings.Finding{a[1], a[0]}
	if !slices.Equal(SummarizeDrainFlags(a, false).Missing, SummarizeDrainFlags(b, false).Missing) {
		t.Error("flag order depends on finding order")
	}
}
