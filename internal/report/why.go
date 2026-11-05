package report

import "github.com/Veinar/draincheck/internal/findings"

// whyOf returns the Why of the case f names, or "" when f names no case the
// catalog holds. Only a case has a Why; a finding of a rule that has not moved
// to cases carries its reason in its description.
func whyOf(cat findings.Catalog, f findings.Finding) string {
	c, _ := cat.Lookup(f)
	return c.Why
}
