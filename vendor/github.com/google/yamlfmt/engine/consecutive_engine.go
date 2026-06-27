// Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package engine

import (
	"fmt"
	"os"

	"github.com/google/yamlfmt"
	"github.com/google/yamlfmt/internal/logger"
)

// Engine that will process each file one by one consecutively.
type ConsecutiveEngine struct {
	LineSepCharacter string
	Formatter        yamlfmt.Formatter
	ContinueOnError  bool
	OutputFormat     EngineOutputFormat
	Quiet            bool
	Verbose          bool
}

func (e *ConsecutiveEngine) FormatContent(content []byte) ([]byte, error) {
	return e.Formatter.Format(content)
}

func (e *ConsecutiveEngine) Format(paths []string) (fmt.Stringer, error) {
	formatDiffs, formatErrs := e.formatAll(paths)

	// Debug format diff output. Manually check for the debug code
	// to be active since the diff string construction can be
	// performance intensive, thus don't want to calculate it if
	// it's not needed.
	if logger.DebugCodeIsActive(logger.DebugCodeDiffs) {
		logger.Debug(
			logger.DebugCodeDiffs,
			fmt.Sprintf("The following files were modified:\n%s", formatDiffs.StrOutput()),
		)
	}

	if len(formatErrs) > 0 {
		if e.ContinueOnError {
			fmt.Print(formatErrs)
			fmt.Println("Continuing...")
		} else {
			return nil, formatErrs
		}
	}
	applyErr := formatDiffs.ApplyAll()
	if applyErr != nil {
		return nil, applyErr
	}
	return getEngineOutput(e.OutputFormat, yamlfmt.OperationFormat, formatDiffs, e.Quiet, e.Verbose)
}

func (e *ConsecutiveEngine) Lint(paths []string) (fmt.Stringer, error) {
	formatDiffs, formatErrs := e.formatAll(paths)
	if len(formatErrs) > 0 {
		return nil, formatErrs
	}
	if formatDiffs.ChangedCount() == 0 {
		return nil, nil
	}
	return getEngineOutput(e.OutputFormat, yamlfmt.OperationLint, formatDiffs, e.Quiet, e.Verbose)
}

func (e *ConsecutiveEngine) DryRun(paths []string) (fmt.Stringer, error) {
	formatDiffs, formatErrs := e.formatAll(paths)
	if len(formatErrs) > 0 {
		return nil, formatErrs
	}
	if formatDiffs.ChangedCount() == 0 {
		return nil, nil
	}
	return getEngineOutput(e.OutputFormat, yamlfmt.OperationDry, formatDiffs, e.Quiet, e.Verbose)
}

func (e *ConsecutiveEngine) formatAll(paths []string) (yamlfmt.FileDiffs, FormatErrors) {
	formatDiffs := yamlfmt.FileDiffs{}
	formatErrs := FormatErrors{}
	for _, path := range paths {
		fileDiff, err := e.formatFileContent(path)
		if err != nil {
			formatErrs = append(formatErrs, wrapFormatError(path, err))
			continue
		}
		formatDiffs.Add(fileDiff)
	}
	return formatDiffs, formatErrs
}

func (e *ConsecutiveEngine) formatFileContent(path string) (*yamlfmt.FileDiff, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	formatted, err := e.FormatContent(content)
	if err != nil {
		return nil, err
	}
	return &yamlfmt.FileDiff{
		Path: path,
		Diff: &yamlfmt.FormatDiff{
			Original:  content,
			Formatted: formatted,
			LineSep:   e.LineSepCharacter,
		},
	}, nil
}
