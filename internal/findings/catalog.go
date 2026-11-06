package findings

// Catalog maps CaseKey values to the cases of every rule.
type Catalog map[string]Case

// Lookup returns the case a finding names. It reports false when the finding
// names no case or the catalog does not hold it; a nil Catalog finds nothing.
func (c Catalog) Lookup(f Finding) (Case, bool) {
	if f.Case == "" {
		return Case{}, false
	}
	cs, ok := c[CaseKey(f.RuleID, f.Case)]
	return cs, ok
}
