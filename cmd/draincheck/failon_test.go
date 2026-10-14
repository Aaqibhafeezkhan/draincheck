package main

import (
	"testing"

	"github.com/Veinar/draincheck/internal/findings"
)

func TestShouldFail(t *testing.T) {
	of := func(sevs ...findings.Severity) []findings.Finding {
		var fs []findings.Finding
		for _, s := range sevs {
			fs = append(fs, findings.Finding{Severity: s})
		}
		return fs
	}
	tests := []struct {
		name      string
		fs        []findings.Finding
		threshold string
		want      bool
	}{
		{"no findings, any", nil, failOnAny, false},
		{"blocked trips blocked", of(findings.Blocked), failOnBlocked, true},
		{"risk does not trip blocked", of(findings.Risk), failOnBlocked, false},
		{"unknown does not trip blocked", of(findings.Unknown), failOnBlocked, false},
		{"risk trips risk", of(findings.Risk), failOnRisk, true},
		{"blocked trips risk", of(findings.Blocked), failOnRisk, true},
		{"unknown does not trip risk", of(findings.Unknown), failOnRisk, false},
		{"unknown trips any", of(findings.Unknown), failOnAny, true},
		{"mixed list, blocked threshold", of(findings.Risk, findings.Blocked), failOnBlocked, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldFail(tt.fs, tt.threshold); got != tt.want {
				t.Fatalf("shouldFail = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidFailOn(t *testing.T) {
	for _, v := range failOnValues {
		if err := validFailOn(v); err != nil {
			t.Errorf("validFailOn(%q) = %v", v, err)
		}
	}
	if err := validFailOn("unknown"); err == nil {
		t.Error("validFailOn(\"unknown\") = nil, want error")
	}
}
