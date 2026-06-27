package dupword

import (
	"strings"

	"github.com/Abirdcfly/dupword"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.DupWordSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		cfg = map[string]any{
			"keyword":       strings.Join(settings.Keywords, ","),
			"ignore":        strings.Join(settings.Ignore, ","),
			"comments-only": settings.CommentsOnly,
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(dupword.NewAnalyzer()).
		WithDesc("Checks for duplicate words in the source code").
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
