package bidichk

import (
	"strings"

	"github.com/breml/bidichk/pkg/bidichk"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.BiDiChkSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		var opts []string

		if settings.LeftToRightEmbedding {
			opts = append(opts, "LEFT-TO-RIGHT-EMBEDDING")
		}
		if settings.RightToLeftEmbedding {
			opts = append(opts, "RIGHT-TO-LEFT-EMBEDDING")
		}
		if settings.PopDirectionalFormatting {
			opts = append(opts, "POP-DIRECTIONAL-FORMATTING")
		}
		if settings.LeftToRightOverride {
			opts = append(opts, "LEFT-TO-RIGHT-OVERRIDE")
		}
		if settings.RightToLeftOverride {
			opts = append(opts, "RIGHT-TO-LEFT-OVERRIDE")
		}
		if settings.LeftToRightIsolate {
			opts = append(opts, "LEFT-TO-RIGHT-ISOLATE")
		}
		if settings.RightToLeftIsolate {
			opts = append(opts, "RIGHT-TO-LEFT-ISOLATE")
		}
		if settings.FirstStrongIsolate {
			opts = append(opts, "FIRST-STRONG-ISOLATE")
		}
		if settings.PopDirectionalIsolate {
			opts = append(opts, "POP-DIRECTIONAL-ISOLATE")
		}

		cfg = map[string]any{
			"disallowed-runes": strings.Join(opts, ","),
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(bidichk.NewAnalyzer()).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeSyntax)
}
