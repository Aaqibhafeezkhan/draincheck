package analyzer

import (
	"bytes"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

// stubRule returns a fixed set of findings.
type stubRule struct {
	id  string
	out []findings.Finding
}

func (r stubRule) ID() string                        { return r.id }
func (r stubRule) Evaluate(Input) []findings.Finding { return r.out }

func finding(ruleID string, ns, kind, name string) findings.Finding {
	return findings.Finding{
		RuleID:   ruleID,
		Severity: findings.Risk,
		Resource: findings.ResourceRef{Namespace: ns, Kind: kind, Name: name},
	}
}

func TestRunSortsFindings(t *testing.T) {
	a := stubRule{"A", []findings.Finding{finding("A", "web", "Pod", "z"), finding("A", "billing", "Pod", "p")}}
	b := stubRule{"B", []findings.Finding{finding("B", "", "Node", "worker-01")}}

	got := Run(nil, Input{}, a, b)

	want := []findings.Finding{
		finding("B", "", "Node", "worker-01"),
		finding("A", "billing", "Pod", "p"),
		finding("A", "web", "Pod", "z"),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Run() = %+v, want %+v", got, want)
	}
}

func TestRunIndependentOfRuleOrder(t *testing.T) {
	a := stubRule{"A", []findings.Finding{finding("A", "web", "Pod", "p")}}
	b := stubRule{"B", []findings.Finding{finding("B", "web", "Pod", "p")}}

	if got, want := Run(nil, Input{}, a, b), Run(nil, Input{}, b, a); !reflect.DeepEqual(got, want) {
		t.Errorf("rule order changed output\n got: %+v\nwant: %+v", got, want)
	}
}

func TestRunPassesInputToRules(t *testing.T) {
	var seen Input
	r := captureRule{seen: &seen}
	in := Input{Targets: []string{"worker-01", "worker-02"}}

	Run(nil, in, r)

	if !reflect.DeepEqual(seen.Targets, in.Targets) {
		t.Errorf("rule saw targets %v, want %v", seen.Targets, in.Targets)
	}
}

type captureRule struct{ seen *Input }

func (captureRule) ID() string { return "capture" }
func (r captureRule) Evaluate(in Input) []findings.Finding {
	*r.seen = in
	return nil
}

func TestRunNoRulesOrFindings(t *testing.T) {
	if got := Run(nil, Input{}); len(got) != 0 {
		t.Errorf("Run() with no rules = %+v, want empty", got)
	}
	if got := Run(nil, Input{}, stubRule{id: "A"}); len(got) != 0 {
		t.Errorf("Run() with silent rule = %+v, want empty", got)
	}
}

func TestRunLogsEachRule(t *testing.T) {
	a := stubRule{"A", []findings.Finding{finding("A", "web", "Pod", "z"), finding("A", "web", "Pod", "y")}}
	b := stubRule{id: "B"}
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	Run(logger, Input{}, a, b)

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	want := []string{
		`msg="rule start" rule=A`,
		`msg="rule done" rule=A findings=2`,
		`msg="rule start" rule=B`,
		`msg="rule done" rule=B findings=0`,
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d log lines, want %d:\n%s", len(lines), len(want), buf.String())
	}
	for i, w := range want {
		if !strings.Contains(lines[i], w) {
			t.Errorf("line %d = %q, want it to contain %q", i+1, lines[i], w)
		}
	}
}
