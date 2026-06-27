package migrate

import (
	"cmp"
	"slices"

	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/ptr"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/versionone"
)

type LinterInfo struct {
	Name             string   `json:"name"`
	Presets          []string `json:"inPresets"`
	Slow             bool     `json:"isSlow,omitempty"`
	Default          bool     `json:"enabledByDefault,omitempty"`
	AlternativeNames []string `json:"alternativeNames,omitempty"`
}

func (l *LinterInfo) isName(name string) bool {
	if name == l.Name {
		return true
	}

	return slices.Contains(l.AlternativeNames, name)
}

func (l *LinterInfo) hasPresets(names []string) bool {
	for _, name := range names {
		if slices.Contains(l.Presets, name) {
			return true
		}
	}

	return false
}

func ProcessEffectiveLinters(old versionone.Linters) (enable, disable []string) {
	switch {
	case ptr.Deref(old.DisableAll):
		return disableAllFilter(old), nil

	case ptr.Deref(old.EnableAll):
		return nil, enableAllFilter(old)

	default:
		return defaultLintersFilter(old)
	}
}

func ProcessEffectiveFormatters(old versionone.Linters) []string {
	enabled, disabled := ProcessEffectiveLinters(old)

	if ptr.Deref(old.EnableAll) {
		var formatterNames []string

		for _, f := range getAllFormatterNames() {
			if !slices.Contains(disabled, f) {
				formatterNames = append(formatterNames, f)
			}
		}

		return formatterNames
	}

	return onlyFormatterNames(enabled)
}

// disableAllFilter generates the value of `enable` when `disable-all` is `true`.
func disableAllFilter(old versionone.Linters) []string {
	// Note:
	// - disable-all + enable-all
	// 		=> impossible (https://github.com/golangci/golangci-lint/blob/e1eb4cb2c7fba29b5831b63e454844d83c692874/pkg/config/linters.go#L38)
	// - disable-all + disable
	// 		=> impossible (https://github.com/golangci/golangci-lint/blob/e1eb4cb2c7fba29b5831b63e454844d83c692874/pkg/config/linters.go#L47)

	// if disable-all -> presets fast enable
	// - presets fast enable: (presets - [slow]) + enable => effective enable + none
	// - presets fast: presets - slow => effective enable + none
	// - presets fast enable: presets + enable => effective enable + none
	// - enable => effective enable + none
	// - fast => nothing
	// - fast enable: enable => effective enable + none

	// (presets - [slow]) + enable => effective enable + none
	names := toNames(
		slices.Concat(
			filter(
				allLinters(), onlyPresets(old), keepFast(old), // presets - [slow]
			),
			allEnabled(old, allLinters()), // + enable
		),
	)

	return slices.Concat(names, unknownLinterNames(old.Enable, allLinters()))
}

// enableAllFilter generates the value of `disable` when `enable-all` is `true`.
func enableAllFilter(old versionone.Linters) []string {
	// Note:
	// - enable-all + disable-all
	// 		=> impossible (https://github.com/golangci/golangci-lint/blob/e1eb4cb2c7fba29b5831b63e454844d83c692874/pkg/config/linters.go#L38)
	// - enable-all + enable + fast=false
	// 		=> impossible (https://github.com/golangci/golangci-lint/blob/e1eb4cb2c7fba29b5831b63e454844d83c692874/pkg/config/linters.go#L52)
	// - enable-all + enable + fast=true
	// 		=> possible (https://github.com/golangci/golangci-lint/blob/e1eb4cb2c7fba29b5831b63e454844d83c692874/pkg/config/linters.go#L51)

	// if enable-all -> presets fast enable disable
	// - presets fast enable disable: all - fast - enable + disable => effective disable + all
	// - presets fast disable: all - fast + disable => effective disable + all
	// - presets fast: all - fast => effective disable + all
	// - presets disable: disable => effective disable + all
	// - disable => effective disable + all
	// - fast: all - fast => effective disable + all
	// - fast disable: all - fast + disable => effective disable + all

	// all - [fast] - enable + disable => effective disable + all
	names := toNames(
		slices.Concat(
			removeLinters(
				filter(
					allLinters(), keepSlow(old), // all - fast
				),
				allEnabled(old, allLinters()), // - enable
			),
			allDisabled(old, allLinters()), // + disable
		),
	)

	return slices.Concat(names, unknownLinterNames(old.Disable, allLinters()))
}

// defaultLintersFilter generates the values of `enable` and `disable` when using default linters.
func defaultLintersFilter(old versionone.Linters) (enable, disable []string) {
	// Note:
	// - a linter cannot be inside `enable` and `disable` in the same configuration
	// 		=> https://github.com/golangci/golangci-lint/blob/e1eb4cb2c7fba29b5831b63e454844d83c692874/pkg/config/linters.go#L66

	// if default -> presets fast disable
	// - presets > fast > disable > enable => effective enable + disable + standard
	//    - (default - fast) - enable + disable => effective disable
	//    - presets - slow + enable - default - [effective disable] => effective enable
	// - presets + fast + disable => effective enable + disable + standard
	//    - (default - fast) + disable => effective disable
	//    - presets - slow - default - [effective disable] => effective enable
	// - presets + fast + enable
	//    - (default - fast) - enable => effective disable
	//    - presets - slow + enable - default - [effective disable] => effective enable
	// - presets + fast
	//    - (default - fast) => effective disable
	//    - presets - slow - default - [effective disable] => effective enable
	// - presets + disable
	//    - default + disable => effective disable
	//    - presets - default - [effective disable] => effective enable
	// - presets + enable
	//    - default - enable => effective disable
	//    - presets + enable - default - [effective disable] => effective enable
	// - disable
	//    - default + disable => effective disable
	//    - default - [effective disable] => effective enable
	// - enable
	//    - default - enable => effective disable
	//    - enable - default - [effective disable] => effective enable
	// - fast
	//    - default - fast => effective disable
	//    - default - [effective disable] => effective enable
	// - fast + disable
	//    - (default - fast) + disable => effective disable
	//    - default - [effective disable] => effective enable
	// - fast + enable
	//    - (default - fast) - enable => effective disable
	//    - enable - default - [effective disable] => effective enable

	disabledLinters := defaultLintersDisableFilter(old)

	enabledLinters := defaultLintersEnableFilter(old, disabledLinters)

	enabled := toNames(enabledLinters)
	disabled := toNames(disabledLinters)

	return slices.Concat(enabled, unknownLinterNames(old.Enable, allLinters())),
		slices.Concat(disabled, unknownLinterNames(old.Disable, allLinters()))
}

// defaultLintersEnableFilter generates the value of `enable` when using default linters.
func defaultLintersEnableFilter(old versionone.Linters, effectiveDisabled []LinterInfo) []LinterInfo {
	// presets - slow + enable - default - [effective disable] => effective enable
	return removeLinters(
		filter(
			slices.Concat(
				filter(
					allLinters(), onlyPresets(old), keepFast(old), // presets - slow
				),
				allEnabled(old, allLinters()), // + enable
			),
			notDefault, // - default
		),
		effectiveDisabled, // - [effective disable]
	)
}

// defaultLintersDisableFilter generates the value of `disable` when using default linters.
func defaultLintersDisableFilter(old versionone.Linters) []LinterInfo {
	// (default - fast) - enable + disable => effective disable
	return slices.Concat(
		removeLinters(
			filter(allLinters(), onlyDefault, keepSlow(old)), // (default - fast)
			allEnabled(old, allLinters()),                    // - enable
		),
		allDisabled(old, allLinters()), // + disable
	)
}

func allLinters() []LinterInfo {
	return []LinterInfo{
		{
			Name:    "asasalint",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "asciicheck",
			Presets: []string{"bugs", "style"},
		},
		{
			Name:    "bidichk",
			Presets: []string{"bugs"},
		},
		{
			Name:    "bodyclose",
			Presets: []string{"performance", "bugs"},
			Slow:    true,
		},
		{
			Name:    "canonicalheader",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "containedctx",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "contextcheck",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "copyloopvar",
			Presets: []string{"style"},
		},
		{
			Name:    "cyclop",
			Presets: []string{"complexity"},
		},
		{
			Name:    "decorder",
			Presets: []string{"style"},
		},
		{
			Name:    "depguard",
			Presets: []string{"style", "import", "module"},
		},
		{
			Name:    "dogsled",
			Presets: []string{"style"},
		},
		{
			Name:    "dupl",
			Presets: []string{"style"},
		},
		{
			Name:    "dupword",
			Presets: []string{"comment"},
		},
		{
			Name:    "durationcheck",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "errcheck",
			Presets: []string{"bugs", "error"},
			Slow:    true,
			Default: true,
		},
		{
			Name:    "errchkjson",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "errname",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "errorlint",
			Presets: []string{"bugs", "error"},
			Slow:    true,
		},
		{
			Name:    "exhaustive",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "exhaustruct",
			Presets: []string{"style", "test"},
			Slow:    true,
		},
		{
			Name:    "exptostd",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "forbidigo",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "forcetypeassert",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "fatcontext",
			Presets: []string{"performance"},
			Slow:    true,
		},
		{
			Name:    "funlen",
			Presets: []string{"complexity"},
		},
		{
			Name:    "gci",
			Presets: []string{"format", "import"},
		},
		{
			Name:    "ginkgolinter",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "gocheckcompilerdirectives",
			Presets: []string{"bugs"},
		},
		{
			Name:    "gochecknoglobals",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "gochecknoinits",
			Presets: []string{"style"},
		},
		{
			Name:    "gochecksumtype",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "gocognit",
			Presets: []string{"complexity"},
		},
		{
			Name:    "goconst",
			Presets: []string{"style"},
		},
		{
			Name:    "gocritic",
			Presets: []string{"style", "metalinter"},
			Slow:    true,
		},
		{
			Name:    "gocyclo",
			Presets: []string{"complexity"},
		},
		{
			Name:    "godot",
			Presets: []string{"style", "comment"},
		},
		{
			Name:    "godox",
			Presets: []string{"style", "comment"},
		},
		{
			Name:             "err113",
			Presets:          []string{"style", "error"},
			Slow:             true,
			AlternativeNames: []string{"goerr113"},
		},
		{
			Name:    "gofmt",
			Presets: []string{"format"},
		},
		{
			Name:    "gofumpt",
			Presets: []string{"format"},
		},
		{
			Name:    "goheader",
			Presets: []string{"style"},
		},
		{
			Name:    "goimports",
			Presets: []string{"format", "import"},
		},
		{
			Name:             "mnd",
			Presets:          []string{"style"},
			AlternativeNames: []string{"gomnd"},
		},
		{
			Name:    "gomoddirectives",
			Presets: []string{"style", "module"},
		},
		{
			Name:    "gomodguard",
			Presets: []string{"style", "import", "module"},
		},
		{
			Name:    "goprintffuncname",
			Presets: []string{"style"},
		},
		{
			Name:             "gosec",
			Presets:          []string{"bugs"},
			Slow:             true,
			AlternativeNames: []string{"gas"},
		},
		{
			Name:             "gosimple",
			Presets:          []string{"style"},
			Slow:             true,
			Default:          true,
			AlternativeNames: []string{"megacheck"},
		},
		{
			Name:    "gosmopolitan",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:             "govet",
			Presets:          []string{"bugs", "metalinter"},
			Slow:             true,
			Default:          true,
			AlternativeNames: []string{"vet", "vetshadow"},
		},
		{
			Name:    "grouper",
			Presets: []string{"style"},
		},
		{
			Name:    "iface",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "importas",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "inamedparam",
			Presets: []string{"style"},
		},
		{
			Name:    "ineffassign",
			Presets: []string{"unused"},
			Default: true,
		},
		{
			Name:    "interfacebloat",
			Presets: []string{"style"},
		},
		{
			Name:    "intrange",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "ireturn",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "lll",
			Presets: []string{"style"},
		},
		{
			Name:             "loggercheck",
			Presets:          []string{"style", "bugs"},
			Slow:             true,
			AlternativeNames: []string{"logrlint"},
		},
		{
			Name:    "maintidx",
			Presets: []string{"complexity"},
		},
		{
			Name:    "makezero",
			Presets: []string{"style", "bugs"},
			Slow:    true,
		},
		{
			Name:    "mirror",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "misspell",
			Presets: []string{"style", "comment"},
		},
		{
			Name:    "musttag",
			Presets: []string{"style", "bugs"},
			Slow:    true,
		},
		{
			Name:    "nakedret",
			Presets: []string{"style"},
		},
		{
			Name:    "nestif",
			Presets: []string{"complexity"},
		},
		{
			Name:    "nilerr",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "nilnesserr",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "nilnil",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "nlreturn",
			Presets: []string{"style"},
		},
		{
			Name:    "noctx",
			Presets: []string{"performance", "bugs"},
			Slow:    true,
		},
		{
			Name:    "nonamedreturns",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "nosprintfhostport",
			Presets: []string{"style"},
		},
		{
			Name:    "paralleltest",
			Presets: []string{"style", "test"},
			Slow:    true,
		},
		{
			Name:    "perfsprint",
			Presets: []string{"performance"},
			Slow:    true,
		},
		{
			Name:    "prealloc",
			Presets: []string{"performance"},
		},
		{
			Name:    "predeclared",
			Presets: []string{"style"},
		},
		{
			Name:    "promlinter",
			Presets: []string{"style"},
		},
		{
			Name:    "protogetter",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "reassign",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "recvcheck",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "revive",
			Presets: []string{"style", "metalinter"},
			Slow:    true,
		},
		{
			Name:    "rowserrcheck",
			Presets: []string{"bugs", "sql"},
			Slow:    true,
		},
		{
			Name:    "sloglint",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "sqlclosecheck",
			Presets: []string{"bugs", "sql"},
			Slow:    true,
		},
		{
			Name:    "spancheck",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:             "staticcheck",
			Presets:          []string{"bugs", "metalinter"},
			Slow:             true,
			Default:          true,
			AlternativeNames: []string{"megacheck"},
		},
		{
			Name:    "stylecheck",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "tagalign",
			Presets: []string{"style"},
		},
		{
			Name:    "tagliatelle",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "testableexamples",
			Presets: []string{"test"},
		},
		{
			Name:    "testifylint",
			Presets: []string{"test", "bugs"},
			Slow:    true,
		},
		{
			Name:    "testpackage",
			Presets: []string{"style", "test"},
		},
		{
			Name:    "thelper",
			Presets: []string{"test"},
			Slow:    true,
		},
		{
			Name:    "tparallel",
			Presets: []string{"style", "test"},
			Slow:    true,
		},
		{
			Name:    "unconvert",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "unparam",
			Presets: []string{"unused"},
			Slow:    true,
		},
		{
			Name:             "unused",
			Presets:          []string{"unused"},
			Slow:             true,
			Default:          true,
			AlternativeNames: []string{"megacheck"},
		},
		{
			Name:    "usestdlibvars",
			Presets: []string{"style"},
		},
		{
			Name:    "usetesting",
			Presets: []string{"test"},
			Slow:    true,
		},
		{
			Name:    "varnamelen",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "wastedassign",
			Presets: []string{"style"},
			Slow:    true,
		},
		{
			Name:    "whitespace",
			Presets: []string{"style"},
		},
		{
			Name:    "wrapcheck",
			Presets: []string{"style", "error"},
			Slow:    true,
		},
		{
			Name:    "wsl",
			Presets: []string{"style"},
		},
		{
			Name:    "zerologlint",
			Presets: []string{"bugs"},
			Slow:    true,
		},
		{
			Name:    "nolintlint",
			Presets: []string{"style"},
		},
	}
}

func toNames(linters []LinterInfo) []string {
	var results []string

	for _, linter := range linters {
		results = append(results, linter.Name)
	}

	return Unique(results)
}

func removeLinters(linters, toRemove []LinterInfo) []LinterInfo {
	return slices.DeleteFunc(linters, func(info LinterInfo) bool {
		return slices.ContainsFunc(toRemove, func(en LinterInfo) bool {
			return info.Name == en.Name
		})
	})
}

func allEnabled(old versionone.Linters, linters []LinterInfo) []LinterInfo {
	var results []LinterInfo

	for _, linter := range linters {
		if slices.ContainsFunc(old.Enable, linter.isName) {
			results = append(results, linter)
		}
	}

	return results
}

func allDisabled(old versionone.Linters, linters []LinterInfo) []LinterInfo {
	var results []LinterInfo

	for _, linter := range linters {
		if slices.ContainsFunc(old.Disable, linter.isName) {
			results = append(results, linter)
		}
	}

	return results
}

func filter(linters []LinterInfo, fns ...fnFilter) []LinterInfo {
	var results []LinterInfo

	for _, linter := range linters {
		if mergeFilters(linter, fns) {
			results = append(results, linter)
		}
	}

	return results
}

func mergeFilters(linter LinterInfo, fns []fnFilter) bool {
	for _, fn := range fns {
		if !fn(linter) {
			return false
		}
	}

	return true
}

type fnFilter func(linter LinterInfo) bool

func onlyPresets(old versionone.Linters) fnFilter {
	return func(linter LinterInfo) bool {
		return linter.hasPresets(old.Presets)
	}
}

func onlyDefault(linter LinterInfo) bool {
	return linter.Default
}

func notDefault(linter LinterInfo) bool {
	return !linter.Default
}

func keepFast(old versionone.Linters) fnFilter {
	return func(linter LinterInfo) bool {
		if !ptr.Deref(old.Fast) {
			return true
		}

		return !linter.Slow
	}
}

func keepSlow(old versionone.Linters) fnFilter {
	return func(linter LinterInfo) bool {
		if !ptr.Deref(old.Fast) {
			return false
		}

		return linter.Slow
	}
}

func unknownLinterNames(names []string, linters []LinterInfo) []string {
	deprecatedLinters := []string{
		"deadcode",
		"execinquery",
		"exhaustivestruct",
		"exportloopref",
		"golint",
		"ifshort",
		"interfacer",
		"maligned",
		"nosnakecase",
		"scopelint",
		"structcheck",
		"tenv",
		"typecheck",
		"varcheck",
	}

	var results []string

	for _, name := range names {
		found := slices.ContainsFunc(linters, func(l LinterInfo) bool {
			return l.isName(name)
		})

		if !found {
			if slices.Contains(deprecatedLinters, name) {
				continue
			}

			results = append(results, name)
		}
	}

	return Unique(results)
}

func convertStaticcheckLinterNames(names []string) []string {
	var results []string

	for _, name := range names {
		if slices.Contains([]string{"stylecheck", "gosimple"}, name) {
			results = append(results, "staticcheck")
			continue
		}

		results = append(results, name)
	}

	return Unique(results)
}

func convertDisabledStaticcheckLinterNames(names []string) []string {
	removeStaticcheck := slices.Contains(names, "staticcheck") && slices.Contains(names, "stylecheck") && slices.Contains(names, "gosimple")

	var results []string

	for _, name := range names {
		if removeStaticcheck && slices.Contains([]string{"stylecheck", "gosimple", "staticcheck"}, name) {
			results = append(results, "staticcheck")
			continue
		}

		if slices.Contains([]string{"stylecheck", "gosimple"}, name) {
			continue
		}

		results = append(results, name)
	}

	return Unique(results)
}

func onlyLinterNames(names []string) []string {
	formatters := []string{"gci", "gofmt", "gofumpt", "goimports"}

	var results []string

	for _, name := range names {
		if !slices.Contains(formatters, name) {
			results = append(results, name)
		}
	}

	return results
}

func onlyFormatterNames(names []string) []string {
	formatters := getAllFormatterNames()

	var results []string

	for _, name := range names {
		if slices.Contains(formatters, name) {
			results = append(results, name)
		}
	}

	return results
}

func convertAlternativeNames(names []string) []string {
	altNames := map[string]string{
		"gas":       "gosec",
		"goerr113":  "err113",
		"gomnd":     "mnd",
		"logrlint":  "loggercheck",
		"megacheck": "staticcheck",
		"vet":       "govet",
		"vetshadow": "govet",
	}

	var results []string

	for _, name := range names {
		if name == "typecheck" {
			continue
		}

		if n, ok := altNames[name]; ok {
			results = append(results, n)
			continue
		}

		results = append(results, name)
	}

	return Unique(results)
}

func Unique[S ~[]E, E cmp.Ordered](s S) S {
	return slices.Compact(slices.Sorted(slices.Values(s)))
}

func getAllFormatterNames() []string {
	return []string{"gci", "gofmt", "gofumpt", "goimports"}
}
