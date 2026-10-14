package report

import (
	"encoding/json"
	"io"
	"slices"

	"github.com/Veinar/draincheck/internal/findings"
)

// SchemaVersion is the version of the JSON document written by WriteJSON.
const SchemaVersion = "v1"

type jsonReport struct {
	SchemaVersion string             `json:"schema_version"`
	Findings      []findings.Finding `json:"findings"`
}

// WriteJSON writes fs to w as an indented JSON document. Findings are sorted
// on a copy; an empty input is written as an empty array, never null.
func WriteJSON(w io.Writer, fs []findings.Finding) error {
	sorted := slices.Clone(fs)
	if sorted == nil {
		sorted = []findings.Finding{}
	}
	findings.Sort(sorted)

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(jsonReport{SchemaVersion: SchemaVersion, Findings: sorted})
}
