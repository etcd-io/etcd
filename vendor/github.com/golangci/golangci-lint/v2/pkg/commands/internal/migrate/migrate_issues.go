package migrate

import (
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/versionone"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/versiontwo"
)

func toIssues(old *versionone.Config) versiontwo.Issues {
	return versiontwo.Issues{
		MaxIssuesPerLinter: old.Issues.MaxIssuesPerLinter,
		MaxSameIssues:      old.Issues.MaxSameIssues,
		UniqByLine:         old.Issues.UniqByLine,
		DiffFromRevision:   old.Issues.DiffFromRevision,
		DiffFromMergeBase:  old.Issues.DiffFromMergeBase,
		DiffPatchFilePath:  old.Issues.DiffPatchFilePath,
		WholeFiles:         old.Issues.WholeFiles,
		Diff:               old.Issues.Diff,
		NeedFix:            old.Issues.NeedFix,
	}
}
