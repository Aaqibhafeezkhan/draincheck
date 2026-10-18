package analyzer

// DefaultRules returns every rule draincheck runs, in a stable order.
func DefaultRules() []Rule {
	return []Rule{
		PDBDisruptionBlocked{},
		PDBOverlappingSelectors{},
		PDBStaleStatus{},
		PodOwnership{},
		MissingResourceRequests{},
	}
}
