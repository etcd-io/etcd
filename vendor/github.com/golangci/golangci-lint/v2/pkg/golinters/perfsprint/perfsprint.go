package perfsprint

import (
	"github.com/catenacyber/perfsprint/analyzer"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.PerfSprintSettings) *goanalysis.Linter {
	cfg := map[string]any{
		"fiximports": false,
	}

	if settings != nil {
		// NOTE: The option `ignore-tests` is not handled because it should be managed with `linters.exclusions.rules`

		cfg["integer-format"] = settings.IntegerFormat
		cfg["int-conversion"] = settings.IntConversion

		cfg["error-format"] = settings.ErrorFormat
		cfg["err-error"] = settings.ErrError
		cfg["errorf"] = settings.ErrorF

		cfg["string-format"] = settings.StringFormat
		cfg["sprintf1"] = settings.SprintF1
		cfg["strconcat"] = settings.StrConcat

		cfg["bool-format"] = settings.BoolFormat
		cfg["hex-format"] = settings.HexFormat

		cfg["concat-loop"] = settings.ConcatLoop
		cfg["loop-other-ops"] = settings.LoopOtherOps
	}

	return goanalysis.
		NewLinterFromAnalyzer(analyzer.New()).
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
