package main

import (
	"fmt"
	"slices"

	"github.com/Veinar/draincheck/internal/findings"
)

// Supported values for --fail-on.
const (
	failOnBlocked = "blocked"
	failOnRisk    = "risk"
	failOnAny     = "any"
)

var failOnValues = []string{failOnBlocked, failOnRisk, failOnAny}

func validFailOn(v string) error {
	if !slices.Contains(failOnValues, v) {
		return fmt.Errorf("invalid --fail-on %q: must be %q, %q or %q", v, failOnBlocked, failOnRisk, failOnAny)
	}
	return nil
}

// shouldFail reports whether fs holds a finding at or above the threshold:
// "blocked" trips on BLOCKED, "risk" on BLOCKED or RISK, "any" on every
// finding including UNKNOWN.
func shouldFail(fs []findings.Finding, threshold string) bool {
	for _, f := range fs {
		switch threshold {
		case failOnAny:
			return true
		case failOnRisk:
			if f.Severity == findings.Blocked || f.Severity == findings.Risk {
				return true
			}
		default:
			if f.Severity == findings.Blocked {
				return true
			}
		}
	}
	return false
}
