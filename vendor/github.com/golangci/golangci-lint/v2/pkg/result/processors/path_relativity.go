package processors

import (
	"fmt"
	"path/filepath"

	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

var _ Processor = (*PathRelativity)(nil)

// PathRelativity computes [result.Issue.RelativePath] and [result.Issue.WorkingDirectoryRelativePath],
// based on the base path.
type PathRelativity struct {
	log              logutils.Log
	basePath         string
	workingDirectory string
}

func NewPathRelativity(log logutils.Log, basePath string) (*PathRelativity, error) {
	wd, err := fsutils.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting working directory: %w", err)
	}

	return &PathRelativity{
		log:              log.Child(logutils.DebugKeyPathRelativity),
		basePath:         basePath,
		workingDirectory: wd,
	}, nil
}

func (*PathRelativity) Name() string {
	return "path_relativity"
}

func (p *PathRelativity) Process(issues []*result.Issue) ([]*result.Issue, error) {
	return transformIssues(issues, func(issue *result.Issue) *result.Issue {
		newIssue := *issue

		var err error
		newIssue.RelativePath, err = filepath.Rel(p.basePath, issue.FilePath())
		if err != nil {
			p.log.Warnf("Getting relative path (basepath): %v", err)
			return nil
		}

		newIssue.WorkingDirectoryRelativePath, err = filepath.Rel(p.workingDirectory, issue.FilePath())
		if err != nil {
			p.log.Warnf("Getting relative path (wd): %v", err)
			return nil
		}

		return &newIssue
	}), nil
}

func (*PathRelativity) Finish() {}
