package processors

import (
	"path/filepath"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

var _ Processor = (*PathPrettifier)(nil)

// PathPrettifier modifies report file path to be relative to the base path.
// Also handles the `output.path-prefix` option.
type PathPrettifier struct {
	cfg *config.Output

	log logutils.Log
}

func NewPathPrettifier(log logutils.Log, cfg *config.Output) *PathPrettifier {
	return &PathPrettifier{
		cfg: cfg,
		log: log.Child(logutils.DebugKeyPathPrettifier),
	}
}

func (*PathPrettifier) Name() string {
	return "path_prettifier"
}

func (p *PathPrettifier) Process(issues []*result.Issue) ([]*result.Issue, error) {
	if p.cfg.PathMode == fsutils.OutputPathModeAbsolute {
		return issues, nil
	}

	return transformIssues(issues, func(issue *result.Issue) *result.Issue {
		newIssue := issue

		if p.cfg.PathPrefix == "" {
			newIssue.Pos.Filename = issue.RelativePath
		} else {
			newIssue.Pos.Filename = filepath.Join(p.cfg.PathPrefix, issue.RelativePath)
		}

		return newIssue
	}), nil
}

func (*PathPrettifier) Finish() {}
