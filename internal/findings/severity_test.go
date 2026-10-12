package findings

import "testing"

func TestSeverityValid(t *testing.T) {
	for _, s := range []Severity{Blocked, Risk, Unknown} {
		if !s.Valid() {
			t.Errorf("%q should be valid", s)
		}
	}
	for _, s := range []Severity{"SAFE", ""} {
		if s.Valid() {
			t.Errorf("%q should be invalid", s)
		}
	}
}
