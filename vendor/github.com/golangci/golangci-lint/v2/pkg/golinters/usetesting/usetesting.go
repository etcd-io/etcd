package usetesting

import (
	"github.com/ldez/usetesting"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.UseTestingSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		cfg = map[string]any{
			"contextbackground": settings.ContextBackground,
			"contexttodo":       settings.ContextTodo,
			"oschdir":           settings.OSChdir,
			"osmkdirtemp":       settings.OSMkdirTemp,
			"ossetenv":          settings.OSSetenv,
			"ostempdir":         settings.OSTempDir,
			"oscreatetemp":      settings.OSCreateTemp,
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(usetesting.NewAnalyzer()).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
