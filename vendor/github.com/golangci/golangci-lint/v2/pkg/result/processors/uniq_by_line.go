package processors

import (
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const uniqByLineLimit = 1

var _ Processor = (*UniqByLine)(nil)

// UniqByLine filters reports to keep only one report by line of code.
type UniqByLine struct {
	fileLineCounter fileLineCounter
	enabled         bool
}

func NewUniqByLine(enable bool) *UniqByLine {
	return &UniqByLine{
		fileLineCounter: fileLineCounter{},
		enabled:         enable,
	}
}

func (*UniqByLine) Name() string {
	return "uniq_by_line"
}

func (p *UniqByLine) Process(issues []*result.Issue) ([]*result.Issue, error) {
	if !p.enabled {
		return issues, nil
	}

	return filterIssuesUnsafe(issues, p.shouldPassIssue), nil
}

func (*UniqByLine) Finish() {}

func (p *UniqByLine) shouldPassIssue(issue *result.Issue) bool {
	if p.fileLineCounter.GetCount(issue) == uniqByLineLimit {
		return false
	}

	p.fileLineCounter.Increment(issue)

	return true
}

type fileLineCounter map[string]map[int]int

func (f fileLineCounter) GetCount(issue *result.Issue) int {
	return f.getCounter(issue)[issue.Line()]
}

func (f fileLineCounter) Increment(issue *result.Issue) {
	f.getCounter(issue)[issue.Line()]++
}

func (f fileLineCounter) getCounter(issue *result.Issue) map[int]int {
	lc := f[issue.FilePath()]

	if lc == nil {
		lc = map[int]int{}
		f[issue.FilePath()] = lc
	}

	return lc
}
