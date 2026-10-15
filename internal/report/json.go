package report

import (
	"encoding/json"
	"io"
	"slices"
	"time"

	"github.com/Veinar/draincheck/internal/findings"
)

// SchemaVersion is the version of the JSON document written by WriteJSON.
const SchemaVersion = "v1"

type jsonReport struct {
	SchemaVersion string             `json:"schema_version"`
	GeneratedAt   string             `json:"generated_at"`
	Disclaimer    string             `json:"disclaimer"`
	Findings      []findings.Finding `json:"findings"`
}

// WriteJSON writes fs to w as an indented JSON document. taken is the time of
// the snapshot the findings describe (inventory.Snapshot.Timestamp), never the
// wall clock, so the output is reproducible. Findings are sorted on a copy; an
// empty input is written as an empty array, never null.
func WriteJSON(w io.Writer, fs []findings.Finding, taken time.Time) error {
	sorted := slices.Clone(fs)
	if sorted == nil {
		sorted = []findings.Finding{}
	}
	findings.Sort(sorted)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(jsonReport{
		SchemaVersion: SchemaVersion,
		GeneratedAt:   formatTimestamp(taken),
		Disclaimer:    Disclaimer(taken),
		Findings:      sorted,
	})
}
