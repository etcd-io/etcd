package varnamelen

import (
	"strconv"
	"strings"

	"github.com/blizzy78/varnamelen"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.VarnamelenSettings) *goanalysis.Linter {
	var cfg map[string]any

	if settings != nil {
		cfg = map[string]any{
			"checkReceiver":      strconv.FormatBool(settings.CheckReceiver),
			"checkReturn":        strconv.FormatBool(settings.CheckReturn),
			"checkTypeParam":     strconv.FormatBool(settings.CheckTypeParam),
			"ignoreNames":        strings.Join(settings.IgnoreNames, ","),
			"ignoreTypeAssertOk": strconv.FormatBool(settings.IgnoreTypeAssertOk),
			"ignoreMapIndexOk":   strconv.FormatBool(settings.IgnoreMapIndexOk),
			"ignoreChanRecvOk":   strconv.FormatBool(settings.IgnoreChanRecvOk),
			"ignoreDecls":        strings.Join(settings.IgnoreDecls, ","),
		}

		if settings.MaxDistance > 0 {
			cfg["maxDistance"] = strconv.Itoa(settings.MaxDistance)
		}

		if settings.MinNameLength > 0 {
			cfg["minNameLength"] = strconv.Itoa(settings.MinNameLength)
		}
	}

	return goanalysis.
		NewLinterFromAnalyzer(varnamelen.NewAnalyzer()).
		WithDesc("checks that the length of a variable's name matches its scope").
		WithConfig(cfg).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
