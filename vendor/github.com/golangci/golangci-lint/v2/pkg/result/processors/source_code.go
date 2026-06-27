package processors

import (
	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

var _ Processor = (*SourceCode)(nil)

// SourceCode modifies displayed information based on [result.Issue.GetLineRange()].
//
// This is used:
//   - to display the "UnderLinePointer".
//   - in some rare cases to display multiple lines instead of one (ex: `dupl`)
//
// It requires to use [fsutils.LineCache] ([fsutils.FileCache]) to get the file information before the fixes.
type SourceCode struct {
	lineCache *fsutils.LineCache
	log       logutils.Log
}

func NewSourceCode(lc *fsutils.LineCache, log logutils.Log) *SourceCode {
	return &SourceCode{
		lineCache: lc,
		log:       log,
	}
}

func (SourceCode) Name() string {
	return "source_code"
}

func (p SourceCode) Process(issues []*result.Issue) ([]*result.Issue, error) {
	return transformIssues(issues, p.transform), nil
}

func (SourceCode) Finish() {}

func (p SourceCode) transform(issue *result.Issue) *result.Issue {
	newIssue := *issue

	lineRange := issue.GetLineRange()
	for lineNumber := lineRange.From; lineNumber <= lineRange.To; lineNumber++ {
		line, err := p.lineCache.GetLine(issue.FilePath(), lineNumber)
		if err != nil {
			p.log.Warnf("Failed to get line %d for file %s: %s",
				lineNumber, issue.FilePath(), err)

			return issue
		}

		newIssue.SourceLines = append(newIssue.SourceLines, line)
	}

	return &newIssue
}
