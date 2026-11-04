package report

import "github.com/Veinar/draincheck/internal/findings"

// remediationOf returns the remediation text to print for f: the one in the
// catalog when f names a case the catalog holds, otherwise the text the finding
// carries itself. The fallback serves the rules that have not moved to cases.
func remediationOf(cat findings.Catalog, f findings.Finding) string {
	if c, ok := cat.Lookup(f); ok {
		return c.Remediation
	}
	return f.Remediation
}
