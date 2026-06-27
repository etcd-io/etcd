package model

import "slices"

// RuleSet represents an immutable set of rule names.
//
// A zero rule set represents an empty set.
type RuleSet struct {
	m map[Rule]struct{}
}

// Merge combines the rules in the current and the given rule sets into a new
// set.
func (rs RuleSet) Merge(another RuleSet) RuleSet {
	result := RuleSet{
		m: make(map[Rule]struct{}, len(rs.m)+len(another.m)),
	}
	for r := range rs.m {
		result.m[r] = struct{}{}
	}
	for r := range another.m {
		result.m[r] = struct{}{}
	}
	return result
}

// Add returns a new rule set containing the rules in the current set and the
// rules provided as arguments.
func (rs RuleSet) Add(rules ...Rule) RuleSet {
	result := RuleSet{
		m: make(map[Rule]struct{}, len(rs.m)+len(rules)),
	}
	for r := range rs.m {
		result.m[r] = struct{}{}
	}
	for _, r := range rules {
		result.m[r] = struct{}{}
	}
	return result
}

// Remove returns a new rule set containing the rules in the current set
// excluding those provided as arguments.
func (rs RuleSet) Remove(rules ...Rule) RuleSet {
	result := RuleSet{
		m: make(map[Rule]struct{}, len(rs.m)),
	}
	for r := range rs.m {
		result.m[r] = struct{}{}
	}
	for _, r := range rules {
		delete(result.m, r)
	}
	return result
}

// Has determines whether the given rule is in the set.
func (rs RuleSet) Has(rule Rule) bool {
	_, ok := rs.m[rule]
	return ok
}

// HasCommonsWith indicates at least one rule is in common with the given rule
// set.
//
// If the given set is empty/zero, the method will return false.
func (rs RuleSet) HasCommonsWith(another RuleSet) bool {
	for r := range another.m {
		if _, ok := rs.m[r]; ok {
			return true
		}
	}
	return false
}

// IsSupersetOf indicates that all rules in the given set are in the current
// set.
//
// If the given set is empty/zero, the method will return true.
func (rs RuleSet) IsSupersetOf(another RuleSet) bool {
	for r := range another.m {
		if _, ok := rs.m[r]; !ok {
			return false
		}
	}
	return true
}

// List returns a slice of the rules in the set.
func (rs RuleSet) List() []Rule {
	rules := make([]Rule, 0, len(rs.m))
	for r := range rs.m {
		rules = append(rules, r)
	}
	slices.Sort(rules)
	return rules
}
