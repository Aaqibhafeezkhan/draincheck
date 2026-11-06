package report

import "github.com/Veinar/draincheck/internal/findings"

// remediationOf returns the remediation of the case f names, or "" when f names
// no case the catalog holds. A finding carries no remediation of its own: the
// text belongs to the case and is written once.
func remediationOf(cat findings.Catalog, f findings.Finding) string {
	c, _ := cat.Lookup(f)
	return c.Remediation
}
