package importas

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/julz/importas"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
)

func New(settings *config.ImportAsSettings) *goanalysis.Linter {
	analyzer := importas.Analyzer

	return goanalysis.
		NewLinterFromAnalyzer(analyzer).
		WithContextSetter(func(lintCtx *linter.Context) {
			if settings == nil {
				return
			}
			if len(settings.Alias) == 0 {
				lintCtx.Log.Infof("importas settings found, but no aliases listed. List aliases under alias: key.")
			}

			if err := analyzer.Flags.Set("no-unaliased", strconv.FormatBool(settings.NoUnaliased)); err != nil {
				lintCtx.Log.Errorf("failed to parse configuration: %v", err)
			}

			if err := analyzer.Flags.Set("no-extra-aliases", strconv.FormatBool(settings.NoExtraAliases)); err != nil {
				lintCtx.Log.Errorf("failed to parse configuration: %v", err)
			}

			uniqPackages := make(map[string]config.ImportAsAlias)
			uniqAliases := make(map[string]config.ImportAsAlias)
			for _, a := range settings.Alias {
				if a.Pkg == "" {
					lintCtx.Log.Errorf("invalid configuration, empty package: pkg=%s alias=%s", a.Pkg, a.Alias)
					continue
				}

				if v, ok := uniqPackages[a.Pkg]; ok {
					lintCtx.Log.Errorf("invalid configuration, multiple aliases for the same package: pkg=%s aliases=[%s,%s]", a.Pkg, a.Alias, v.Alias)
				} else {
					uniqPackages[a.Pkg] = a
				}

				// Skips the duplication check when:
				// - the alias is empty.
				// - the alias is a regular expression replacement pattern (ie. contains `$`).
				v, ok := uniqAliases[a.Alias]
				if ok && a.Alias != "" && !strings.Contains(a.Alias, "$") {
					lintCtx.Log.Errorf("invalid configuration, multiple packages with the same alias: alias=%s packages=[%s,%s]", a.Alias, a.Pkg, v.Pkg)
				} else {
					uniqAliases[a.Alias] = a
				}

				err := analyzer.Flags.Set("alias", fmt.Sprintf("%s:%s", a.Pkg, a.Alias))
				if err != nil {
					lintCtx.Log.Errorf("failed to parse configuration: %v", err)
				}
			}
		}).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}
