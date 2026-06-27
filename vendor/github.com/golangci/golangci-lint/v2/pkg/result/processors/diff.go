package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/golangci/revgrep"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const envGolangciDiffProcessorPatch = "GOLANGCI_DIFF_PROCESSOR_PATCH"

var _ Processor = (*Diff)(nil)

// Diff filters issues based on options `new`, `new-from-rev`, etc.
//
// Uses `git`.
// The paths inside the patch are relative to the path where git is run (the same location where golangci-lint is run).
//
// Warning: it doesn't use `path-prefix` option.
type Diff struct {
	onlyNew       bool
	fromRev       string
	fromMergeBase string
	patchFilePath string
	wholeFiles    bool
	patch         string
}

func NewDiff(cfg *config.Issues) *Diff {
	return &Diff{
		onlyNew:       cfg.Diff,
		fromRev:       cfg.DiffFromRevision,
		fromMergeBase: cfg.DiffFromMergeBase,
		patchFilePath: cfg.DiffPatchFilePath,
		wholeFiles:    cfg.WholeFiles,
		patch:         os.Getenv(envGolangciDiffProcessorPatch),
	}
}

func (*Diff) Name() string {
	return "diff"
}

func (p *Diff) Process(issues []*result.Issue) ([]*result.Issue, error) {
	if !p.onlyNew && p.fromRev == "" && p.fromMergeBase == "" && p.patchFilePath == "" && p.patch == "" {
		return issues, nil
	}

	var patchReader io.Reader
	switch {
	case p.patchFilePath != "":
		patch, err := os.ReadFile(p.patchFilePath)
		if err != nil {
			return nil, fmt.Errorf("can't read from patch file %s: %w", p.patchFilePath, err)
		}

		patchReader = bytes.NewReader(patch)

	case p.patch != "":
		patchReader = strings.NewReader(p.patch)
	}

	checker := revgrep.Checker{
		Patch:        patchReader,
		RevisionFrom: p.fromRev,
		MergeBase:    p.fromMergeBase,
		WholeFiles:   p.wholeFiles,
	}

	err := checker.Prepare(context.Background())
	if err != nil {
		return nil, fmt.Errorf("can't prepare diff by revgrep: %w", err)
	}

	return transformIssues(issues, func(issue *result.Issue) *result.Issue {
		if issue.FromLinter == typeCheckName {
			// Never hide typechecking errors.
			return issue
		}

		hunkPos, isNew := checker.IsNew(issue.WorkingDirectoryRelativePath, issue.Line())
		if !isNew {
			return nil
		}

		newIssue := *issue
		newIssue.HunkPos = hunkPos

		return &newIssue
	}), nil
}

func (*Diff) Finish() {}
