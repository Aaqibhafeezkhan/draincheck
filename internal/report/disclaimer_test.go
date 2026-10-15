package report

import (
	"strings"
	"testing"
	"time"
)

func TestDisclaimerNamesSnapshotTimeAndDisclaimsGuarantee(t *testing.T) {
	got := Disclaimer(snapshotTime)

	for _, want := range []string{"2026-10-10T21:00:00Z", "does not guarantee", "may drift"} {
		if !strings.Contains(got, want) {
			t.Errorf("disclaimer %q is missing %q", got, want)
		}
	}
}

func TestDisclaimerUsesUTC(t *testing.T) {
	local := snapshotTime.In(time.FixedZone("CEST", 2*60*60))

	if Disclaimer(local) != Disclaimer(snapshotTime) {
		t.Errorf("disclaimer depends on the zone: %q vs %q", Disclaimer(local), Disclaimer(snapshotTime))
	}
}
