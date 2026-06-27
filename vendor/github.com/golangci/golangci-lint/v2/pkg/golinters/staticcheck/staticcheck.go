package staticcheck

import (
	"slices"
	"strings"
	"unicode"

	"golang.org/x/tools/go/analysis"
	"honnef.co/go/tools/analysis/lint"
	scconfig "honnef.co/go/tools/config"
	"honnef.co/go/tools/quickfix"
	"honnef.co/go/tools/simple"
	"honnef.co/go/tools/staticcheck"
	"honnef.co/go/tools/stylecheck"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

var (
	debugf  = logutils.Debug(logutils.DebugKeyStaticcheck)
	isDebug = logutils.HaveDebugTag(logutils.DebugKeyStaticcheck)
)

func New(settings *config.StaticCheckSettings) *goanalysis.Linter {
	cfg := createConfig(settings)

	// `scconfig.Analyzer` is a singleton.
	scconfig.Analyzer.Run = func(_ *analysis.Pass) (any, error) {
		return cfg, nil
	}

	allAnalyzers := slices.Concat(staticcheck.Analyzers, stylecheck.Analyzers, simple.Analyzers, quickfix.Analyzers)

	analyzers := setupAnalyzers(allAnalyzers, cfg.Checks)

	if isDebug {
		allAnalyzerNames := extractAnalyzerNames(allAnalyzers)
		slices.Sort(allAnalyzerNames)
		debugf("All available checks (%d): %s", len(allAnalyzers), strings.Join(allAnalyzerNames, ","))

		var cfgAnalyzerNames []string
		for _, a := range analyzers {
			cfgAnalyzerNames = append(cfgAnalyzerNames, a.Name)
		}
		slices.Sort(cfgAnalyzerNames)
		debugf("Enabled by config checks (%d): %s", len(analyzers), strings.Join(cfgAnalyzerNames, ","))

		debugf("staticcheck configuration: %#v", cfg)
	}

	return goanalysis.NewLinter(
		"staticcheck",
		"It's the set of rules from staticcheck.",
		analyzers,
		nil,
	).WithLoadMode(goanalysis.LoadModeTypesInfo)
}

func createConfig(settings *config.StaticCheckSettings) *scconfig.Config {
	defaultChecks := []string{"all", "-ST1000", "-ST1003", "-ST1016", "-ST1020", "-ST1021", "-ST1022"}

	var cfg *scconfig.Config

	if settings == nil || !settings.HasConfiguration() {
		return &scconfig.Config{
			Checks:                  defaultChecks,
			Initialisms:             scconfig.DefaultConfig.Initialisms,
			DotImportWhitelist:      scconfig.DefaultConfig.DotImportWhitelist,
			HTTPStatusCodeWhitelist: scconfig.DefaultConfig.HTTPStatusCodeWhitelist,
		}
	}

	cfg = &scconfig.Config{
		Checks:                  settings.Checks,
		Initialisms:             settings.Initialisms,
		DotImportWhitelist:      settings.DotImportWhitelist,
		HTTPStatusCodeWhitelist: settings.HTTPStatusCodeWhitelist,
	}

	if cfg.Checks == nil {
		cfg.Checks = defaultChecks
	}

	if cfg.Initialisms == nil {
		cfg.Initialisms = append(cfg.Initialisms, scconfig.DefaultConfig.Initialisms...)
	}

	if cfg.DotImportWhitelist == nil {
		cfg.DotImportWhitelist = append(cfg.DotImportWhitelist, scconfig.DefaultConfig.DotImportWhitelist...)
	}

	if cfg.HTTPStatusCodeWhitelist == nil {
		cfg.HTTPStatusCodeWhitelist = append(cfg.HTTPStatusCodeWhitelist, scconfig.DefaultConfig.HTTPStatusCodeWhitelist...)
	}

	cfg.Checks = normalizeList(cfg.Checks)
	cfg.Initialisms = normalizeList(cfg.Initialisms)
	cfg.DotImportWhitelist = normalizeList(cfg.DotImportWhitelist)
	cfg.HTTPStatusCodeWhitelist = normalizeList(cfg.HTTPStatusCodeWhitelist)

	return cfg
}

// https://github.com/dominikh/go-tools/blob/9bf17c0388a65710524ba04c2d821469e639fdc2/config/config.go#L95-L116
func normalizeList(list []string) []string {
	if len(list) > 1 {
		nlist := make([]string, 0, len(list))
		nlist = append(nlist, list[0])
		for i, el := range list[1:] {
			if el != list[i] {
				nlist = append(nlist, el)
			}
		}
		list = nlist
	}

	for _, el := range list {
		if el == "inherit" {
			// This should never happen, because the default config
			// should not use "inherit"
			panic(`unresolved "inherit"`)
		}
	}

	return list
}

func setupAnalyzers(src []*lint.Analyzer, checks []string) []*analysis.Analyzer {
	filter := filterAnalyzerNames(extractAnalyzerNames(src), checks)

	var ret []*analysis.Analyzer
	for _, a := range src {
		if filter[a.Analyzer.Name] {
			ret = append(ret, a.Analyzer)
		}
	}

	return ret
}

func extractAnalyzerNames(analyzers []*lint.Analyzer) []string {
	var names []string
	for _, a := range analyzers {
		names = append(names, a.Analyzer.Name)
	}
	return names
}

// https://github.com/dominikh/go-tools/blob/9bf17c0388a65710524ba04c2d821469e639fdc2/lintcmd/lint.go#L437-L477
//
//nolint:gocritic // Keep the original source code.
func filterAnalyzerNames(analyzers []string, checks []string) map[string]bool {
	allowedChecks := map[string]bool{}

	for _, check := range checks {
		b := true
		if len(check) > 1 && check[0] == '-' {
			b = false
			check = check[1:]
		}

		if check == "*" || check == "all" {
			// Match all
			for _, c := range analyzers {
				allowedChecks[c] = b
			}
		} else if strings.HasSuffix(check, "*") {
			// Glob
			prefix := check[:len(check)-1]
			isCat := strings.IndexFunc(prefix, func(r rune) bool { return unicode.IsNumber(r) }) == -1

			for _, a := range analyzers {
				idx := strings.IndexFunc(a, func(r rune) bool { return unicode.IsNumber(r) })
				if isCat {
					// Glob is S*, which should match S1000 but not SA1000
					cat := a[:idx]
					if prefix == cat {
						allowedChecks[a] = b
					}
				} else {
					// Glob is S1*
					if strings.HasPrefix(a, prefix) {
						allowedChecks[a] = b
					}
				}
			}
		} else {
			// Literal check name
			allowedChecks[check] = b
		}
	}
	return allowedChecks
}
