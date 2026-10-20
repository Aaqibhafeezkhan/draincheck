package analyzer

import "testing"

func TestDefaultRules(t *testing.T) {
	rules := DefaultRules()
	if len(rules) == 0 {
		t.Fatal("no default rules")
	}
	seen := map[string]bool{}
	for _, r := range rules {
		if r.ID() == "" {
			t.Errorf("%T has an empty ID", r)
		}
		if seen[r.ID()] {
			t.Errorf("duplicate rule ID %q", r.ID())
		}
		seen[r.ID()] = true
	}
	for _, id := range []string{"PDB_DISRUPTION_BLOCKED", "PDB_OVERLAPPING_SELECTORS", "PDB_STALE_STATUS", "PDB_SYNC_FAILED", "PDB_BATCH_EXCEEDS_BUDGET", "POD_OWNERSHIP", "MISSING_RESOURCE_REQUESTS"} {
		if !seen[id] {
			t.Errorf("rule %q is not registered", id)
		}
	}
}
