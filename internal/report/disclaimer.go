package report

import (
	"fmt"
	"time"
)

// disclaimerFormat is the mandatory statement from docs/SPEC.md section 3;
// %s is the snapshot timestamp.
const disclaimerFormat = "This report describes the observed cluster state at %s and does not guarantee execution success. Cluster state may drift immediately after analysis."

// Disclaimer returns the statement every report carries, naming the time the
// snapshot was taken. Both formatters use it so the wording never diverges.
func Disclaimer(taken time.Time) string {
	return fmt.Sprintf(disclaimerFormat, formatTimestamp(taken))
}

// formatTimestamp renders t as RFC 3339 in UTC, so output does not depend on
// the local zone of the machine that ran the analysis.
func formatTimestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
