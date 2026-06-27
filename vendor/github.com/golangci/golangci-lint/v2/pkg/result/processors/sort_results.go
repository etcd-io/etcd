package processors

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const (
	orderNameFile     = "file"
	orderNameLinter   = "linter"
	orderNameSeverity = "severity"
)

const (
	less = iota - 1
	equal
	greater
)

var _ Processor = (*SortResults)(nil)

type issueComparator func(a, b *result.Issue) int

// SortResults sorts reports based on criteria:
//   - file names, line numbers, positions
//   - linter names
//   - severity names
type SortResults struct {
	cmps map[string][]issueComparator

	cfg *config.Output
}

func NewSortResults(cfg *config.Output) *SortResults {
	return &SortResults{
		cmps: map[string][]issueComparator{
			// For sorting we are comparing (in next order):
			// file names, line numbers, position, and finally - giving up.
			orderNameFile: {byFileName, byLine, byColumn},
			// For sorting we are comparing: linter name
			orderNameLinter: {byLinter},
			// For sorting we are comparing: severity
			orderNameSeverity: {bySeverity},
		},
		cfg: cfg,
	}
}

func (SortResults) Name() string { return "sort_results" }

// Process is performing sorting of the result issues.
func (p SortResults) Process(issues []*result.Issue) ([]*result.Issue, error) {
	if len(p.cfg.SortOrder) == 0 {
		p.cfg.SortOrder = []string{orderNameLinter, orderNameFile}
	}

	var cmps []issueComparator

	for _, name := range p.cfg.SortOrder {
		c, ok := p.cmps[name]
		if !ok {
			return nil, fmt.Errorf("unsupported sort-order name %q", name)
		}

		cmps = append(cmps, c...)
	}

	comp := mergeComparators(cmps...)

	slices.SortFunc(issues, func(a, b *result.Issue) int {
		return comp(a, b)
	})

	return issues, nil
}

func (SortResults) Finish() {}

func byFileName(a, b *result.Issue) int {
	return strings.Compare(a.FilePath(), b.FilePath())
}

func byLine(a, b *result.Issue) int {
	return numericCompare(a.Line(), b.Line())
}

func byColumn(a, b *result.Issue) int {
	return numericCompare(a.Column(), b.Column())
}

func byLinter(a, b *result.Issue) int {
	return strings.Compare(a.FromLinter, b.FromLinter)
}

func bySeverity(a, b *result.Issue) int {
	return severityCompare(a.Severity, b.Severity)
}

func severityCompare(a, b string) int {
	// The position inside the slice define the importance (lower to higher).
	classic := []string{"low", "medium", "high", "warning", "error"}

	if slices.Contains(classic, a) && slices.Contains(classic, b) {
		return cmp.Compare(slices.Index(classic, a), slices.Index(classic, b))
	}

	if slices.Contains(classic, a) {
		return greater
	}

	if slices.Contains(classic, b) {
		return less
	}

	return strings.Compare(a, b)
}

func numericCompare(a, b int) int {
	// Negative values and zeros are skipped (equal) because they either invalid or  "neutral" (default int value).
	if a <= 0 || b <= 0 {
		return equal
	}

	return cmp.Compare(a, b)
}

func mergeComparators(comps ...issueComparator) issueComparator {
	return func(a, b *result.Issue) int {
		for _, comp := range comps {
			i := comp(a, b)
			if i != equal {
				return i
			}
		}

		return equal
	}
}
