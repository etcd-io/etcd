package processors

import (
	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

var _ Processor = (*MaxFromLinter)(nil)

// MaxFromLinter limits the number of reports from the same linter.
type MaxFromLinter struct {
	linterCounter map[string]int
	limit         int
	log           logutils.Log
	cfg           *config.Config
}

func NewMaxFromLinter(limit int, log logutils.Log, cfg *config.Config) *MaxFromLinter {
	return &MaxFromLinter{
		linterCounter: map[string]int{},
		limit:         limit,
		log:           log,
		cfg:           cfg,
	}
}

func (*MaxFromLinter) Name() string {
	return "max_from_linter"
}

func (p *MaxFromLinter) Process(issues []*result.Issue) ([]*result.Issue, error) {
	if p.limit <= 0 { // no limit
		return issues, nil
	}

	return filterIssuesUnsafe(issues, func(issue *result.Issue) bool {
		p.linterCounter[issue.FromLinter]++ // always inc for stat

		return p.linterCounter[issue.FromLinter] <= p.limit
	}), nil
}

func (p *MaxFromLinter) Finish() {
	walkStringToIntMapSortedByValue(p.linterCounter, func(linter string, count int) {
		if count > p.limit {
			p.log.Infof("%d/%d issues from linter %s were hidden, use --max-issues-per-linter",
				count-p.limit, count, linter)
		}
	})
}
