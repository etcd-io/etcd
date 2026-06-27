package goformatters

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rogpeppe/go-internal/diff"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/goformatters/internal"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

// NewAnalyzer converts a [Formatter] to an [analysis.Analyzer].
func NewAnalyzer(logger logutils.Log, doc string, formatter Formatter) *analysis.Analyzer {
	return &analysis.Analyzer{
		Name: formatter.Name(),
		Doc:  doc,
		Run: func(pass *analysis.Pass) (any, error) {
			for _, file := range pass.Files {
				position, isGoFile := goanalysis.GetGoFilePosition(pass, file)
				if !isGoFile {
					continue
				}

				input, err := os.ReadFile(position.Filename)
				if err != nil {
					return nil, fmt.Errorf("unable to open file %s: %w", position.Filename, err)
				}

				output, err := formatter.Format(position.Filename, input)
				if err != nil {
					return nil, fmt.Errorf("error while running %s: %w", formatter.Name(), err)
				}

				if !bytes.Equal(input, output) {
					newName := filepath.ToSlash(position.Filename)
					oldName := newName + ".orig"

					patch := diff.Diff(oldName, input, newName, output)

					err = internal.ExtractDiagnosticFromPatch(pass, file, patch, logger)
					if err != nil {
						return nil, fmt.Errorf("can't extract issues from %s diff output %q: %w", formatter.Name(), patch, err)
					}
				}
			}

			return nil, nil
		},
	}
}
