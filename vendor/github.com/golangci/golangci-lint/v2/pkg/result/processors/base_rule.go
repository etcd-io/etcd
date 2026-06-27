package processors

import (
	"regexp"
	"slices"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

type baseRule struct {
	text       *regexp.Regexp
	source     *regexp.Regexp
	path       *regexp.Regexp
	pathExcept *regexp.Regexp
	linters    []string
}

// The usage of `regexp.MustCompile()` is safe here,
// because the regular expressions are checked before inside [config.BaseRule.Validate].
func newBaseRule(rule *config.BaseRule, prefix string) baseRule {
	base := baseRule{
		linters: rule.Linters,
	}

	if rule.Text != "" {
		base.text = regexp.MustCompile(prefix + rule.Text)
	}

	if rule.Source != "" {
		base.source = regexp.MustCompile(prefix + rule.Source)
	}

	if rule.Path != "" {
		base.path = regexp.MustCompile(fsutils.NormalizePathInRegex(rule.Path))
	}

	if rule.PathExcept != "" {
		base.pathExcept = regexp.MustCompile(fsutils.NormalizePathInRegex(rule.PathExcept))
	}

	return base
}

func (r *baseRule) isEmpty() bool {
	return r.text == nil && r.source == nil && r.path == nil && r.pathExcept == nil && len(r.linters) == 0
}

func (r *baseRule) match(issue *result.Issue, lines *fsutils.LineCache, log logutils.Log) bool {
	if r.isEmpty() {
		return false
	}
	if r.text != nil && !r.text.MatchString(issue.Text) {
		return false
	}
	if r.path != nil && !r.path.MatchString(issue.RelativePath) {
		return false
	}
	if r.pathExcept != nil && r.pathExcept.MatchString(issue.RelativePath) {
		return false
	}
	if len(r.linters) != 0 && !r.matchLinter(issue) {
		return false
	}

	// the most heavyweight checking last
	if r.source != nil && !r.matchSource(issue, lines, log) {
		return false
	}

	return true
}

func (r *baseRule) matchLinter(issue *result.Issue) bool {
	return slices.Contains(r.linters, issue.FromLinter)
}

func (r *baseRule) matchSource(issue *result.Issue, lineCache *fsutils.LineCache, log logutils.Log) bool {
	sourceLine, errSourceLine := lineCache.GetLine(issue.FilePath(), issue.Line())
	if errSourceLine != nil {
		log.Warnf("Failed to get line %s:%d from line cache: %s", issue.FilePath(), issue.Line(), errSourceLine)
		return false // can't properly match
	}

	return r.source.MatchString(sourceLine)
}

func parseRules[T, V any](rules []T, prefix string, newFn func(*T, string) V) []V {
	if len(rules) == 0 {
		return nil
	}

	parsedRules := make([]V, 0, len(rules))

	for _, r := range rules {
		parsedRules = append(parsedRules, newFn(&r, prefix))
	}

	return parsedRules
}
