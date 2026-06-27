package processors

import (
	"fmt"
	"slices"
	"strings"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

var _ Processor = (*ExclusionRules)(nil)

type ExclusionRules struct {
	log logutils.Log

	lines *fsutils.LineCache

	warnUnused     bool
	skippedCounter map[string]int

	rules []excludeRule
}

func NewExclusionRules(log logutils.Log, lines *fsutils.LineCache, cfg *config.LinterExclusions) *ExclusionRules {
	p := &ExclusionRules{
		log:            log,
		lines:          lines,
		warnUnused:     cfg.WarnUnused,
		skippedCounter: map[string]int{},
	}

	excludeRules := slices.Concat(cfg.Rules, getLinterExclusionPresets(cfg.Presets))

	p.rules = parseRules(excludeRules, "", newExcludeRule)

	for _, rule := range p.rules {
		if rule.internalReference == "" {
			p.skippedCounter[rule.String()] = 0
		}
	}

	return p
}

func (*ExclusionRules) Name() string {
	return "exclusion_rules"
}

func (p *ExclusionRules) Process(issues []*result.Issue) ([]*result.Issue, error) {
	if len(p.rules) == 0 {
		return issues, nil
	}

	return filterIssues(issues, func(issue *result.Issue) bool {
		for _, rule := range p.rules {
			if !rule.match(issue, p.lines, p.log) {
				continue
			}

			// Ignore default rules.
			if rule.internalReference == "" {
				p.skippedCounter[rule.String()]++
			}

			return false
		}

		return true
	}), nil
}

func (p *ExclusionRules) Finish() {
	for rule, count := range p.skippedCounter {
		if p.warnUnused && count == 0 {
			p.log.Warnf("Skipped %d issues by rules: [%s]", count, rule)
		} else {
			p.log.Infof("Skipped %d issues by rules: [%s]", count, rule)
		}
	}
}

type excludeRule struct {
	baseRule

	// For compatibility with exclude-use-default/include.
	internalReference string `mapstructure:"-"`
}

func newExcludeRule(rule *config.ExcludeRule, prefix string) excludeRule {
	return excludeRule{
		baseRule:          newBaseRule(&rule.BaseRule, prefix),
		internalReference: rule.InternalReference,
	}
}

func (e excludeRule) String() string {
	var msg []string

	if e.text != nil && e.text.String() != "" {
		msg = append(msg, fmt.Sprintf("Text: %q", e.text))
	}

	if e.source != nil && e.source.String() != "" {
		msg = append(msg, fmt.Sprintf("Source: %q", e.source))
	}

	if e.path != nil && e.path.String() != "" {
		msg = append(msg, fmt.Sprintf("Path: %q", e.path))
	}

	if e.pathExcept != nil && e.pathExcept.String() != "" {
		msg = append(msg, fmt.Sprintf("Path Except: %q", e.pathExcept))
	}

	if len(e.linters) > 0 {
		msg = append(msg, fmt.Sprintf("Linters: %q", strings.Join(e.linters, ", ")))
	}

	return strings.Join(msg, ", ")
}
