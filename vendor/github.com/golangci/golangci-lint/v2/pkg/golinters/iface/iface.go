package iface

import (
	"slices"

	"github.com/uudashr/iface/identical"
	"github.com/uudashr/iface/opaque"
	"github.com/uudashr/iface/unexported"
	"github.com/uudashr/iface/unused"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
)

func New(settings *config.IfaceSettings) *goanalysis.Linter {
	var conf map[string]map[string]any
	if settings != nil {
		conf = settings.Settings
	}

	return goanalysis.NewLinter(
		"iface",
		"Detect the incorrect use of interfaces, helping developers avoid interface pollution.",
		analyzersFromSettings(settings),
		conf,
	).WithLoadMode(goanalysis.LoadModeTypesInfo)
}

func analyzersFromSettings(settings *config.IfaceSettings) []*analysis.Analyzer {
	allAnalyzers := map[string]*analysis.Analyzer{
		"identical":  identical.Analyzer,
		"unused":     unused.Analyzer,
		"opaque":     opaque.Analyzer,
		"unexported": unexported.Analyzer,
	}

	if settings == nil || len(settings.Enable) == 0 {
		// Default enable `identical` analyzer only
		return []*analysis.Analyzer{identical.Analyzer}
	}

	var analyzers []*analysis.Analyzer
	for _, name := range uniqueNames(settings.Enable) {
		if _, ok := allAnalyzers[name]; !ok {
			// skip unknown analyzer
			continue
		}

		analyzers = append(analyzers, allAnalyzers[name])
	}

	return analyzers
}

func uniqueNames(names []string) []string {
	slices.Sort(names)
	return slices.Compact(names)
}
