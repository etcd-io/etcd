package nlreturn

import (
	"github.com/ssgreg/nlreturn/v2/pkg/nlreturn"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.NlreturnSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		cfg = map[string]any{
			"block-size": settings.BlockSize,
		}
	}
	return goanalysis.
		NewLinterFromAnalyzer(nlreturn.NewAnalyzer()).
		WithDesc("Checks for a new line before return and branch statements to increase code clarity").
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
