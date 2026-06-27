package reassign

import (
	"fmt"
	"strings"

	"github.com/curioswitch/go-reassign"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.ReassignSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil && len(settings.Patterns) > 0 {
		cfg = map[string]any{
			reassign.FlagPattern: fmt.Sprintf("^(%s)$", strings.Join(settings.Patterns, "|")),
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(reassign.NewAnalyzer()).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
