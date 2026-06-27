package goheader

import (
	"go/token"
	"strings"

	goheader "github.com/denis-tingaikin/go-header"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

const linterName = "goheader"

func New(settings *config.GoHeaderSettings, replacer *strings.Replacer) *goanalysis.Linter {
	conf := &goheader.Configuration{}
	if settings != nil {
		conf = &goheader.Configuration{
			Values:       settings.Values,
			Template:     settings.Template,
			TemplatePath: replacer.Replace(settings.TemplatePath),
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(&analysis.Analyzer{
			Name: linterName,
			Doc:  "Check if file header matches to pattern",
			Run: func(pass *analysis.Pass) (any, error) {
				err := runGoHeader(pass, conf)
				if err != nil {
					return nil, err
				}

				return nil, nil
			},
		}).
		WithLoadMode(goanalysis.LoadModeSyntax)
}

func runGoHeader(pass *analysis.Pass, conf *goheader.Configuration) error {
	if conf.TemplatePath == "" && conf.Template == "" {
		// User did not pass template, so then do not run go-header linter
		return nil
	}

	template, err := conf.GetTemplate()
	if err != nil {
		return err
	}

	values, err := conf.GetValues()
	if err != nil {
		return err
	}

	a := goheader.New(goheader.WithTemplate(template), goheader.WithValues(values))

	for _, file := range pass.Files {
		position, isGoFile := goanalysis.GetGoFilePosition(pass, file)
		if !isGoFile {
			continue
		}

		issue := a.Analyze(&goheader.Target{File: file, Path: position.Filename})
		if issue == nil {
			continue
		}

		f := pass.Fset.File(file.Pos())

		commentLine := 1
		var offset int

		// Inspired by https://github.com/denis-tingaikin/go-header/blob/4c75a6a2332f025705325d6c71fff4616aedf48f/analyzer.go#L85-L92
		if len(file.Comments) > 0 && file.Comments[0].Pos() < file.Package {
			if !strings.HasPrefix(file.Comments[0].List[0].Text, "/*") {
				// When the comment is "//" there is a one character offset.
				offset = 1
			}
			commentLine = goanalysis.GetFilePositionFor(pass.Fset, file.Comments[0].Pos()).Line
		}

		// Skip issues related to build directives.
		// https://github.com/denis-tingaikin/go-header/issues/18
		if issue.Location().Position-offset < 0 {
			continue
		}

		diag := analysis.Diagnostic{
			Pos:     f.LineStart(issue.Location().Line+1) + token.Pos(issue.Location().Position-offset), // The position of the first divergence.
			Message: issue.Message(),
		}

		if fix := issue.Fix(); fix != nil {
			current := len(fix.Actual)
			for _, s := range fix.Actual {
				current += len(s)
			}

			start := f.LineStart(commentLine)

			end := start + token.Pos(current)

			header := strings.Join(fix.Expected, "\n") + "\n"

			// Adds an extra line between the package and the header.
			if end == file.Package {
				header += "\n"
			}

			diag.SuggestedFixes = []analysis.SuggestedFix{{
				TextEdits: []analysis.TextEdit{{
					Pos:     start,
					End:     end,
					NewText: []byte(header),
				}},
			}}
		}

		pass.Report(diag)
	}

	return nil
}
