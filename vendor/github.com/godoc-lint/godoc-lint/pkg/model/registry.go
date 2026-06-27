package model

// Registry defines a registry of checkers.
type Registry interface {
	// Add registers a new checker.
	Add(Checker)

	// List returns a slice of the registered checkers.
	List() []Checker

	// GetCoveredRules returns the set of rules covered by the registered
	// checkers.
	GetCoveredRules() RuleSet
}
