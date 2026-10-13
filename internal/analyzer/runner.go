package analyzer

import "github.com/Veinar/draincheck/internal/findings"

// Run evaluates every rule against in and returns all findings sorted with
// findings.Sort, so the result does not depend on rule order or on the order
// in which a rule emits its findings.
func Run(in Input, rules ...Rule) []findings.Finding {
	var all []findings.Finding
	for _, r := range rules {
		all = append(all, r.Evaluate(in)...)
	}
	findings.Sort(all)
	return all
}
