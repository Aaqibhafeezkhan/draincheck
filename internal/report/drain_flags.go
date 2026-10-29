package report

import "github.com/Veinar/draincheck/internal/findings"

// DrainFlags is what the report says about the kubectl drain flags of the plan.
// It is derived from the findings of one run, not from a second look at the
// cluster, so it is only as good as they are.
type DrainFlags struct {
	// Missing are the flags kubectl drain would still demand, in a fixed order:
	// the ones whose absence makes a finding BLOCKED. Never a flag the operator
	// already passed, since passing it lifts the BLOCKED finding. Empty, never
	// nil, when none is missing.
	Missing []string `json:"missing"`
	// EvictionDisabled is true when --disable-eviction was given: pods are
	// deleted, not evicted, and no PodDisruptionBudget is consulted.
	EvictionDisabled bool `json:"eviction_disabled"`
}

// missingFlagOrder is the order Missing is written in.
var missingFlagOrder = []string{"--ignore-daemonsets", "--delete-emptydir-data", "--force"}

// SummarizeDrainFlags derives the flags kubectl drain would still demand from
// the BLOCKED findings in fs: a DaemonSet pod needs --ignore-daemonsets, a pod
// with an emptyDir volume needs --delete-emptydir-data and a pod with no
// controller needs --force. RISK and UNKNOWN findings name no flag, because
// nothing refuses the drain there. evictionDisabled is the --disable-eviction
// flag of the run.
func SummarizeDrainFlags(fs []findings.Finding, evictionDisabled bool) DrainFlags {
	need := map[string]bool{}
	for _, f := range fs {
		if f.Severity != findings.Blocked {
			continue
		}
		switch ev := f.Evidence.(type) {
		case findings.OwnershipEvidence:
			switch ev.Category {
			case findings.OwnershipDaemonSet:
				need["--ignore-daemonsets"] = true
			case findings.OwnershipUnmanaged:
				need["--force"] = true
			}
		case findings.LocalStorageEvidence:
			need["--delete-emptydir-data"] = true
		}
	}
	missing := []string{}
	for _, flag := range missingFlagOrder {
		if need[flag] {
			missing = append(missing, flag)
		}
	}
	return DrainFlags{Missing: missing, EvictionDisabled: evictionDisabled}
}
