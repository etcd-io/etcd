package godot

import (
	"cmp"

	"github.com/tetafro/godot"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.GodotSettings) *goanalysis.Linter {
	var dotSettings godot.Settings

	if settings != nil {
		dotSettings = godot.Settings{
			Scope:   godot.Scope(settings.Scope),
			Exclude: settings.Exclude,
			Period:  settings.Period,
			Capital: settings.Capital,
		}
	}

	dotSettings.Scope = cmp.Or(dotSettings.Scope, godot.DeclScope)

	return goanalysis.
		NewLinterFromAnalyzer(&analysis.Analyzer{
			Name: "godot",
			Doc:  "Check if comments end in a period",
			Run: func(pass *analysis.Pass) (any, error) {
				err := runGodot(pass, dotSettings)
				if err != nil {
					return nil, err
				}

				return nil, nil
			},
		}).
		WithLoadMode(goanalysis.LoadModeSyntax)
}

func runGodot(pass *analysis.Pass, settings godot.Settings) error {
	for _, file := range pass.Files {
		iss, err := godot.Run(file, pass.Fset, settings)
		if err != nil {
			return err
		}

		if len(iss) == 0 {
			continue
		}

		f := pass.Fset.File(file.Pos())

		for _, i := range iss {
			start := f.Pos(i.Pos.Offset)
			end := goanalysis.EndOfLinePos(f, i.Pos.Line)

			pass.Report(analysis.Diagnostic{
				Pos:     start,
				End:     end,
				Message: i.Message,
				SuggestedFixes: []analysis.SuggestedFix{{
					TextEdits: []analysis.TextEdit{{
						Pos:     start,
						End:     end,
						NewText: []byte(i.Replacement),
					}},
				}},
			})
		}
	}

	return nil
}
