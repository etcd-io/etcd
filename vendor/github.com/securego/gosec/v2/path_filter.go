package gosec

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/securego/gosec/v2/issue"
)

// PathExcludeRule defines rules to exclude for specific file paths
type PathExcludeRule struct {
	Path  string   `json:"path"`  // Regex pattern for matching file paths
	Rules []string `json:"rules"` // Rule IDs to exclude. Use "*" to exclude all rules
}

// compiledPathRule is a pre-compiled version of PathExcludeRule for efficient matching
type compiledPathRule struct {
	pathRegex  *regexp.Regexp
	ruleSet    map[string]bool // Set of rule IDs to exclude
	excludeAll bool            // True if "*" was specified in rules
	original   PathExcludeRule // Keep original for error messages
}

// PathExclusionFilter handles filtering of issues based on path and rule combinations
type PathExclusionFilter struct {
	rules []compiledPathRule
}

// NewPathExclusionFilter creates a new filter from the provided exclusion rules.
// Returns an error if any path regex is invalid.
func NewPathExclusionFilter(rules []PathExcludeRule) (*PathExclusionFilter, error) {
	if len(rules) == 0 {
		return &PathExclusionFilter{rules: nil}, nil
	}

	compiled := make([]compiledPathRule, 0, len(rules))

	for i, rule := range rules {
		if rule.Path == "" {
			return nil, fmt.Errorf("exclude-rules[%d]: path cannot be empty", i)
		}

		regex, err := regexp.Compile(rule.Path)
		if err != nil {
			return nil, fmt.Errorf("exclude-rules[%d]: invalid path regex %q: %w", i, rule.Path, err)
		}

		ruleSet := make(map[string]bool)
		excludeAll := false

		for _, ruleID := range rule.Rules {
			ruleID = strings.TrimSpace(ruleID)
			if ruleID == "*" {
				excludeAll = true
			} else if ruleID != "" {
				ruleSet[ruleID] = true
			}
		}

		compiled = append(compiled, compiledPathRule{
			pathRegex:  regex,
			ruleSet:    ruleSet,
			excludeAll: excludeAll,
			original:   rule,
		})
	}

	return &PathExclusionFilter{rules: compiled}, nil
}

// ShouldExclude returns true if the given issue should be excluded based on
// its file path and rule ID
func (f *PathExclusionFilter) ShouldExclude(filePath, ruleID string) bool {
	if f == nil || len(f.rules) == 0 {
		return false
	}

	// Normalize path separators for consistent matching
	normalizedPath := strings.ReplaceAll(filePath, "\\", "/")

	for _, rule := range f.rules {
		if rule.pathRegex.MatchString(normalizedPath) {
			if rule.excludeAll {
				return true
			}
			if rule.ruleSet[ruleID] {
				return true
			}
		}
	}

	return false
}

// FilterIssues applies path-based exclusions to a slice of issues.
// Returns the filtered issues and the count of excluded issues.
func (f *PathExclusionFilter) FilterIssues(issues []*issue.Issue) ([]*issue.Issue, int) {
	if f == nil || len(f.rules) == 0 || len(issues) == 0 {
		return issues, 0
	}

	filtered := make([]*issue.Issue, 0, len(issues))
	excluded := 0

	for _, iss := range issues {
		if f.ShouldExclude(iss.File, iss.RuleID) {
			excluded++
			continue
		}
		filtered = append(filtered, iss)
	}

	return filtered, excluded
}

// ParseCLIExcludeRules parses the CLI format for exclude-rules.
// Format: "path:rule1,rule2;path2:rule3,rule4"
// Example: "cmd/.*:G204,G304;test/.*:G101"
func ParseCLIExcludeRules(input string) ([]PathExcludeRule, error) {
	if input == "" {
		return nil, nil
	}

	var rules []PathExcludeRule

	// Split by semicolon for multiple rules
	parts := strings.Split(input, ";")

	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by colon to separate path and rules
		colonIdx := strings.LastIndex(part, ":")
		if colonIdx == -1 {
			return nil, fmt.Errorf("exclude-rules part %d: missing ':' separator in %q", i+1, part)
		}

		pathPattern := strings.TrimSpace(part[:colonIdx])
		rulesPart := strings.TrimSpace(part[colonIdx+1:])

		if pathPattern == "" {
			return nil, fmt.Errorf("exclude-rules part %d: path pattern cannot be empty", i+1)
		}

		if rulesPart == "" {
			return nil, fmt.Errorf("exclude-rules part %d: rules list cannot be empty", i+1)
		}

		// Split rules by comma
		ruleIDs := strings.Split(rulesPart, ",")
		cleanedRules := make([]string, 0, len(ruleIDs))
		for _, r := range ruleIDs {
			r = strings.TrimSpace(r)
			if r != "" {
				cleanedRules = append(cleanedRules, r)
			}
		}

		if len(cleanedRules) == 0 {
			return nil, fmt.Errorf("exclude-rules part %d: no valid rules specified", i+1)
		}

		rules = append(rules, PathExcludeRule{
			Path:  pathPattern,
			Rules: cleanedRules,
		})
	}

	return rules, nil
}

// MergeExcludeRules combines exclude rules from multiple sources (config file + CLI).
// CLI rules take precedence and are processed first.
func MergeExcludeRules(configRules, cliRules []PathExcludeRule) []PathExcludeRule {
	if len(cliRules) == 0 {
		return configRules
	}
	if len(configRules) == 0 {
		return cliRules
	}

	// CLI rules first, then config rules
	merged := make([]PathExcludeRule, 0, len(cliRules)+len(configRules))
	merged = append(merged, cliRules...)
	merged = append(merged, configRules...)
	return merged
}

// String returns a human-readable representation of the filter
func (f *PathExclusionFilter) String() string {
	if f == nil || len(f.rules) == 0 {
		return "PathExclusionFilter{empty}"
	}

	var parts []string
	for _, rule := range f.rules {
		if rule.excludeAll {
			parts = append(parts, fmt.Sprintf("%s:*", rule.original.Path))
		} else {
			ruleIDs := make([]string, 0, len(rule.ruleSet))
			for id := range rule.ruleSet {
				ruleIDs = append(ruleIDs, id)
			}
			parts = append(parts, fmt.Sprintf("%s:[%s]", rule.original.Path, strings.Join(ruleIDs, ",")))
		}
	}

	return fmt.Sprintf("PathExclusionFilter{%s}", strings.Join(parts, "; "))
}
