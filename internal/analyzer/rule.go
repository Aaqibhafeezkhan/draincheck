package analyzer

import (
	"github.com/Veinar/draincheck/internal/findings"
	"github.com/Veinar/draincheck/internal/inventory"
)

// Input is everything a rule may read: the immutable cluster snapshot and the
// sorted names of the nodes planned for maintenance.
type Input struct {
	Snapshot *inventory.Snapshot
	Targets  []string
}

// Rule is a pure, stateless check. Evaluate must not mutate the cluster or
// keep state between calls; given the same Input it returns the same findings.
type Rule interface {
	// ID is the stable identifier reported as Finding.RuleID.
	ID() string
	// Evaluate returns the findings for in, in any order.
	Evaluate(in Input) []findings.Finding
}
