// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// This file is inspired by go/analysis/internal/checker/checker.go

package processors

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"

	"github.com/golangci/golangci-lint/v2/internal/x/tools/diff"
	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/fsutils"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters/gci"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters/gofmt"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters/gofumpt"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters/goimports"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters/golines"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters/swaggo"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
	"github.com/golangci/golangci-lint/v2/pkg/timeutils"
)

var _ Processor = (*Fixer)(nil)

const filePerm = 0644

// Fixer fixes reports if possible.
// The reports that are not fixed are passed to the next processor.
type Fixer struct {
	cfg       *config.Config
	log       logutils.Log
	fileCache *fsutils.FileCache
	sw        *timeutils.Stopwatch
	formatter *goformatters.MetaFormatter
}

func NewFixer(cfg *config.Config, log logutils.Log, fileCache *fsutils.FileCache, formatter *goformatters.MetaFormatter) *Fixer {
	return &Fixer{
		cfg:       cfg,
		log:       log,
		fileCache: fileCache,
		sw:        timeutils.NewStopwatch("fixer", log),
		formatter: formatter,
	}
}

func (Fixer) Name() string {
	return "fixer"
}

func (p Fixer) Process(issues []*result.Issue) ([]*result.Issue, error) {
	if !p.cfg.Issues.NeedFix {
		return issues, nil
	}

	p.log.Infof("Applying suggested fixes")

	notFixableIssues, err := timeutils.TrackStage(p.sw, "all", func() ([]*result.Issue, error) {
		return p.process(issues)
	})
	if err != nil {
		p.log.Warnf("Failed to fix issues: %v", err)
	}

	p.printStat()

	return notFixableIssues, nil
}

//nolint:funlen,gocyclo // This function should not be split.
func (p Fixer) process(issues []*result.Issue) ([]*result.Issue, error) {
	// filenames / linters / edits
	editsByLinter := make(map[string]map[string][]diff.Edit)

	formatters := []string{gofumpt.Name, goimports.Name, gofmt.Name, gci.Name, golines.Name, swaggo.Name}

	var notFixableIssues []*result.Issue

	toBeFormattedFiles := make(map[string]struct{})

	for i := range issues {
		issue := issues[i]

		if slices.Contains(formatters, issue.FromLinter) {
			toBeFormattedFiles[issue.FilePath()] = struct{}{}
			continue
		}

		if issue.SuggestedFixes == nil || skipNoTextEdit(issue) {
			notFixableIssues = append(notFixableIssues, issue)
			continue
		}

		for _, sf := range issue.SuggestedFixes {
			for _, edit := range sf.TextEdits {
				start, end := edit.Pos, edit.End
				if start > end {
					return nil, fmt.Errorf("%q suggests invalid fix: pos (%v) > end (%v)",
						issue.FromLinter, edit.Pos, edit.End)
				}

				edit := diff.Edit{
					Start: int(start),
					End:   int(end),
					New:   string(edit.NewText),
				}

				if _, ok := editsByLinter[issue.FilePath()]; !ok {
					editsByLinter[issue.FilePath()] = make(map[string][]diff.Edit)
				}

				editsByLinter[issue.FilePath()][issue.FromLinter] = append(editsByLinter[issue.FilePath()][issue.FromLinter], edit)
			}
		}
	}

	// Validate and group the edits to each actual file.
	editsByPath := make(map[string][]diff.Edit)
	for path, linterToEdits := range editsByLinter {
		excludedLinters := make(map[string]struct{})

		linters := slices.Collect(maps.Keys(linterToEdits))

		// Does any linter create conflicting edits?
		for _, linter := range linters {
			edits := linterToEdits[linter]
			if _, invalid := validateEdits(edits); invalid > 0 {
				name, x, y := linter, edits[invalid-1], edits[invalid]
				excludedLinters[name] = struct{}{}

				err := diff3Conflict(path, name, name, []diff.Edit{x}, []diff.Edit{y})
				// TODO(ldez) TUI?
				p.log.Warnf("Changes related to %q are skipped for the file %q: %v",
					name, path, err)
			}
		}

		// Does any pair of different linters create edits that conflict?
		for j := range linters {
			for k := range linters[:j] {
				x, y := linters[j], linters[k]
				if x > y {
					x, y = y, x
				}

				_, foundX := excludedLinters[x]
				_, foundY := excludedLinters[y]
				if foundX || foundY {
					continue
				}

				xedits, yedits := linterToEdits[x], linterToEdits[y]

				combined := slices.Concat(xedits, yedits)

				if _, invalid := validateEdits(combined); invalid > 0 {
					excludedLinters[x] = struct{}{}
					p.log.Warnf("Changes related to %q are skipped for the file %q due to conflicts with %q.", x, path, y)
				}
			}
		}

		var edits []diff.Edit
		for linter := range linterToEdits {
			if _, found := excludedLinters[linter]; !found {
				edits = append(edits, linterToEdits[linter]...)
			}
		}

		editsByPath[path], _ = validateEdits(edits) // remove duplicates. already validated.
	}

	var editError error

	var formattedFiles []string

	// Now we've got a set of valid edits for each file. Apply them.
	for path, edits := range editsByPath {
		contents, err := p.fileCache.GetFileBytes(path)
		if err != nil {
			editError = errors.Join(editError, fmt.Errorf("%s: %w", path, err))
			continue
		}

		out, err := diff.ApplyBytes(contents, edits)
		if err != nil {
			editError = errors.Join(editError, fmt.Errorf("%s: %w", path, err))
			continue
		}

		// Try to format the file.
		out = p.formatter.Format(path, out)

		if err := os.WriteFile(path, out, filePerm); err != nil {
			editError = errors.Join(editError, fmt.Errorf("%s: %w", path, err))
			continue
		}

		formattedFiles = append(formattedFiles, path)
	}

	for path := range toBeFormattedFiles {
		// Skips files already formatted by the previous fix step.
		if !slices.Contains(formattedFiles, path) {
			content, err := p.fileCache.GetFileBytes(path)
			if err != nil {
				p.log.Warnf("Error reading file %s: %v", path, err)
				continue
			}

			out := p.formatter.Format(path, content)

			if err := os.WriteFile(path, out, filePerm); err != nil {
				editError = errors.Join(editError, fmt.Errorf("%s: %w", path, err))
				continue
			}
		}
	}

	return notFixableIssues, editError
}

func (Fixer) Finish() {}

func (p Fixer) printStat() {
	p.sw.PrintStages()
}

func skipNoTextEdit(issue *result.Issue) bool {
	var onlyMessage int
	for _, sf := range issue.SuggestedFixes {
		if len(sf.TextEdits) == 0 {
			onlyMessage++
		}
	}

	return len(issue.SuggestedFixes) == onlyMessage
}

// validateEdits returns a list of edits that is sorted and
// contains no duplicate edits. Returns the index of some
// overlapping adjacent edits if there is one and <0 if the
// edits are valid.
//
//nolint:gocritic // Copy of go/analysis/internal/checker/checker.go
func validateEdits(edits []diff.Edit) ([]diff.Edit, int) {
	if len(edits) == 0 {
		return nil, -1
	}

	equivalent := func(x, y diff.Edit) bool {
		return x.Start == y.Start && x.End == y.End && x.New == y.New
	}

	diff.SortEdits(edits)

	unique := []diff.Edit{edits[0]}

	invalid := -1

	for i := 1; i < len(edits); i++ {
		prev, cur := edits[i-1], edits[i]
		// We skip over equivalent edits without considering them
		// an error. This handles identical edits coming from the
		// multiple ways of loading a package into a
		// *go/packages.Packages for testing, e.g. packages "p" and "p [p.test]".
		if !equivalent(prev, cur) {
			unique = append(unique, cur)
			if prev.End > cur.Start {
				invalid = i
			}
		}
	}
	return unique, invalid
}

// diff3Conflict returns an error describing two conflicting sets of
// edits on a file at path.
// Copy of go/analysis/internal/checker/checker.go
func diff3Conflict(path, xlabel, ylabel string, xedits, yedits []diff.Edit) error {
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	oldlabel, old := "base", string(contents)

	xdiff, err := diff.ToUnified(oldlabel, xlabel, old, xedits, diff.DefaultContextLines)
	if err != nil {
		return err
	}
	ydiff, err := diff.ToUnified(oldlabel, ylabel, old, yedits, diff.DefaultContextLines)
	if err != nil {
		return err
	}

	return fmt.Errorf("conflicting edits from %s and %s on %s\nfirst edits:\n%s\nsecond edits:\n%s",
		xlabel, ylabel, path, xdiff, ydiff)
}
