package gosmopolitan

import (
	"strings"

	"github.com/xen0n/gosmopolitan"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.GosmopolitanSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		cfg = map[string]any{
			"allowtimelocal":  settings.AllowTimeLocal,
			"escapehatches":   strings.Join(settings.EscapeHatches, ","),
			"watchforscripts": strings.Join(settings.WatchForScripts, ","),

			// Should be managed with `linters.exclusions.rules`.
			"lookattests": true,
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(gosmopolitan.NewAnalyzer()).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
