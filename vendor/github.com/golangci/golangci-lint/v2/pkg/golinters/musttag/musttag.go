package musttag

import (
	"go-simpler.org/musttag"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.MustTagSettings) *goanalysis.Linter {
	var funcs []musttag.Func

	if settings != nil {
		for _, fn := range settings.Functions {
			funcs = append(funcs, musttag.Func{
				Name:   fn.Name,
				Tag:    fn.Tag,
				ArgPos: fn.ArgPos,
			})
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(musttag.New(funcs...)).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
