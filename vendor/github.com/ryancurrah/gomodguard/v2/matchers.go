package gomodguard

import (
	"fmt"
	"regexp"
	"strings"
)

// MatchType represents the type of matching to be performed for a module name.
type MatchType string

const (
	// ExactMatch matches a module name exactly.
	ExactMatch MatchType = "exact"
	// PrefixMatch matches a module name by prefix.
	PrefixMatch MatchType = "prefix"
	// RegexMatch matches a module name by regex.
	RegexMatch MatchType = "regex"
)

// Matcher interface for matching module names.
type Matcher interface {
	Match(moduleName string) bool
}

// ExactMatcher matches a module name exactly.
type ExactMatcher struct {
	Target string
}

func (m ExactMatcher) Match(moduleName string) bool {
	return strings.TrimSpace(moduleName) == m.Target
}

// PrefixMatcher matches a module name by prefix.
type PrefixMatcher struct {
	Prefix string
}

// Match returns true if the moduleName starts with the Prefix, ignoring leading/trailing whitespace and case.
func (m PrefixMatcher) Match(moduleName string) bool {
	return strings.HasPrefix(strings.TrimSpace(strings.ToLower(moduleName)), strings.ToLower(m.Prefix))
}

// RegexMatcher matches a module name by regex.
type RegexMatcher struct {
	Regex *regexp.Regexp
}

func (m RegexMatcher) Match(moduleName string) bool {
	if m.Regex == nil {
		return false
	}

	return m.Regex.MatchString(strings.TrimSpace(moduleName))
}

// compileMatcher creates a Matcher based on the match type and pattern.
//
//nolint:ireturn // This factory intentionally returns the Matcher interface.
func compileMatcher(matchType MatchType, pattern string) (Matcher, error) {
	switch matchType {
	case PrefixMatch:
		return PrefixMatcher{Prefix: strings.TrimSpace(pattern)}, nil
	case RegexMatch:
		re, err := regexp.Compile(strings.TrimSpace(pattern))
		if err != nil {
			return nil, err
		}

		return RegexMatcher{Regex: re}, nil
	case ExactMatch, "":
		return ExactMatcher{Target: strings.TrimSpace(pattern)}, nil
	default:
		return nil, fmt.Errorf("unknown match-type %q for pattern %q", matchType, pattern)
	}
}
