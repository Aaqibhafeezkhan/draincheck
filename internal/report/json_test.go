package report

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

var update = flag.Bool("update", false, "rewrite golden files")

// sampleFindings is deliberately out of sorted order.
func sampleFindings() []findings.Finding {
	return []findings.Finding{
		{
			RuleID:      "POD_OWNERSHIP",
			Severity:    findings.Risk,
			Resource:    findings.ResourceRef{Kind: "Pod", Namespace: "monitoring", Name: "agent-x1"},
			Evidence:    findings.OwnershipEvidence{Category: findings.OwnershipDaemonSet, Node: "worker-02", OwnerKind: "DaemonSet", OwnerName: "agent"},
			Description: "Pod monitoring/agent-x1 is managed by a DaemonSet.",
			Remediation: "Drain with --ignore-daemonsets.",
		},
		{
			RuleID:      "PDB_DISRUPTION_BLOCKED",
			Severity:    findings.Blocked,
			Resource:    findings.ResourceRef{Kind: "Pod", Namespace: "billing", Name: "payments-api-0"},
			Evidence:    findings.PDBEvidence{Namespace: "billing", Name: "payments-api-pdb", CurrentHealthy: 3, DesiredHealthy: 3},
			Description: "PodDisruptionBudget billing/payments-api-pdb allows 0 disruptions.",
			Remediation: "Add healthy replicas or relax the budget.",
		},
		{
			RuleID:      "UNKNOWN_CONSTRAINT",
			Severity:    findings.Unknown,
			Resource:    findings.ResourceRef{Kind: "Node", Name: "worker-02"},
			Description: "A constraint could not be evaluated.",
			Remediation: "Check it manually.",
		},
		{
			RuleID:      "MISSING_RESOURCE_REQUESTS",
			Severity:    findings.Risk,
			Resource:    findings.ResourceRef{Kind: "Pod", Namespace: "shop", Name: "web-0"},
			Evidence:    findings.MissingRequestsEvidence{Node: "worker-02", Containers: []findings.MissingRequestsContainer{{Name: "web", MissingCPU: true, MissingMemory: true}}},
			Description: "Pod shop/web-0 has containers without resource requests.",
			Remediation: "Set CPU and memory requests.",
		},
		{
			RuleID:      "PDB_OVERLAPPING_SELECTORS",
			Severity:    findings.Blocked,
			Resource:    findings.ResourceRef{Kind: "Pod", Namespace: "billing", Name: "payments-api-1"},
			Evidence:    findings.OverlappingPDBsEvidence{Namespace: "billing", PDBs: []string{"a-pdb", "b-pdb"}},
			Description: "Pod is selected by 2 PodDisruptionBudgets.",
			Remediation: "Narrow the selectors.",
		},
	}
}

func render(t *testing.T, fs []findings.Finding) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteJSON(&buf, fs); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestWriteJSONGolden(t *testing.T) {
	got := render(t, sampleFindings())

	path := filepath.Join("..", "..", "testdata", "report", "json-v1.golden.json")
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

func TestWriteJSONSchemaVersion(t *testing.T) {
	var doc struct {
		SchemaVersion string `json:"schema_version"`
	}
	if err := json.Unmarshal(render(t, nil), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.SchemaVersion != "v1" {
		t.Errorf("schema_version = %q, want %q", doc.SchemaVersion, "v1")
	}
}

func TestWriteJSONIsOrderIndependent(t *testing.T) {
	fs := sampleFindings()
	want := render(t, fs)

	reversed := slices.Clone(fs)
	slices.Reverse(reversed)
	if got := render(t, reversed); !bytes.Equal(got, want) {
		t.Errorf("output depends on input order\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteJSONDoesNotMutateInput(t *testing.T) {
	fs := sampleFindings()
	before := slices.Clone(fs)

	render(t, fs)

	for i := range fs {
		if fs[i].Resource != before[i].Resource || fs[i].RuleID != before[i].RuleID {
			t.Fatalf("input slice was reordered at index %d", i)
		}
	}
}

func TestWriteJSONEmptyIsEmptyArray(t *testing.T) {
	for name, in := range map[string][]findings.Finding{"nil": nil, "empty": {}} {
		t.Run(name, func(t *testing.T) {
			var doc struct {
				Findings *[]json.RawMessage `json:"findings"`
			}
			if err := json.Unmarshal(render(t, in), &doc); err != nil {
				t.Fatal(err)
			}
			if doc.Findings == nil || len(*doc.Findings) != 0 {
				t.Errorf(`"findings" must be an empty array, got %v`, doc.Findings)
			}
		})
	}
}

func TestWriteJSONEndsWithNewline(t *testing.T) {
	if got := render(t, nil); !bytes.HasSuffix(got, []byte("\n")) {
		t.Errorf("output must end with a newline: %q", got)
	}
}

func TestWriteJSONDoesNotEscapeProse(t *testing.T) {
	fs := []findings.Finding{{
		RuleID:      "R",
		Severity:    findings.Risk,
		Resource:    findings.ResourceRef{Kind: "Pod", Namespace: "ns", Name: "p"},
		Description: "needs cpu < 1 & memory > 64Mi",
	}}

	got := render(t, fs)

	if bytes.Contains(got, []byte(`\u003c`)) || bytes.Contains(got, []byte(`\u0026`)) {
		t.Errorf("prose must not be HTML-escaped: %s", got)
	}
	if !bytes.Contains(got, []byte("cpu < 1 & memory > 64Mi")) {
		t.Errorf("description missing from output: %s", got)
	}
}

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestWriteJSONReturnsWriterError(t *testing.T) {
	want := errors.New("disk full")

	err := WriteJSON(failingWriter{err: want}, sampleFindings())

	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}
