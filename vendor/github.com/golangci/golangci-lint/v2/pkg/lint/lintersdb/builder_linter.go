package lintersdb

import (
	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/golinters"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/arangolint"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/asasalint"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/asciicheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/bidichk"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/bodyclose"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/canonicalheader"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/clickhouselint"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/containedctx"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/contextcheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/copyloopvar"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/cyclop"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/decorder"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/depguard"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/dogsled"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/dupl"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/dupword"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/durationcheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/embeddedstructfieldcheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/err113"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/errcheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/errchkjson"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/errname"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/errorlint"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/exhaustive"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/exhaustruct"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/exptostd"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/fatcontext"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/forbidigo"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/forcetypeassert"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/funcorder"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/funlen"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gci"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/ginkgolinter"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gocheckcompilerdirectives"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gochecknoglobals"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gochecknoinits"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gochecksumtype"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gocognit"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/goconst"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gocritic"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gocyclo"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/godoclint"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/godot"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/godox"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gofmt"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gofumpt"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/goheader"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/goimports"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/golines"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gomoddirectives"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gomodguard"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/goprintffuncname"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gosec"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/gosmopolitan"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/govet"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/grouper"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/iface"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/importas"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/inamedparam"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/ineffassign"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/interfacebloat"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/intrange"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/iotamixing"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/ireturn"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/lll"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/loggercheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/maintidx"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/makezero"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/mirror"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/misspell"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/mnd"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/modernize"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/musttag"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/nakedret"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/nestif"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/nilerr"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/nilnesserr"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/nilnil"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/nlreturn"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/noctx"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/noinlineerr"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/nolintlint"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/nonamedreturns"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/nosprintfhostport"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/paralleltest"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/perfsprint"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/prealloc"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/predeclared"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/promlinter"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/protogetter"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/reassign"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/recvcheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/revive"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/rowserrcheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/sloglint"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/spancheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/sqlclosecheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/staticcheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/swaggo"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/tagalign"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/tagliatelle"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/testableexamples"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/testifylint"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/testpackage"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/thelper"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/tparallel"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/unconvert"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/unparam"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/unqueryvet"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/unused"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/usestdlibvars"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/usetesting"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/varnamelen"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/wastedassign"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/whitespace"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/wrapcheck"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/wsl"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/zerologlint"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
)

// LinterBuilder builds the "internal" linters based on the configuration.
type LinterBuilder struct{}

// NewLinterBuilder creates a new LinterBuilder.
func NewLinterBuilder() *LinterBuilder {
	return &LinterBuilder{}
}

// Build loads all the "internal" linters.
// The configuration is used for the linter settings.
func (LinterBuilder) Build(cfg *config.Config) ([]*linter.Config, error) {
	if cfg == nil {
		return nil, nil
	}

	placeholderReplacer := config.NewPlaceholderReplacer(cfg)

	// The linters are sorted in the alphabetical order (case-insensitive).
	// When a new linter is added the version in `WithSince(...)` must be the next minor version of golangci-lint.
	return []*linter.Config{
		linter.NewConfig(arangolint.New()).
			WithSince("v2.2.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/Crocmagnon/arangolint"),

		linter.NewConfig(asasalint.New(&cfg.Linters.Settings.Asasalint)).
			WithSince("v1.47.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/alingse/asasalint"),

		linter.NewConfig(asciicheck.New()).
			WithSince("v1.26.0").
			WithURL("https://github.com/golangci/asciicheck"),

		linter.NewConfig(bidichk.New(&cfg.Linters.Settings.BiDiChk)).
			WithSince("v1.43.0").
			WithURL("https://github.com/breml/bidichk"),

		linter.NewConfig(bodyclose.New(&cfg.Linters.Settings.BodyClose)).
			WithSince("v1.18.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/timakin/bodyclose"),

		linter.NewConfig(canonicalheader.New()).
			WithSince("v1.58.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/lasiar/canonicalheader"),

		linter.NewConfig(clickhouselint.New()).
			WithSince("v2.12.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/ClickHouse/clickhouse-go-linter"),

		linter.NewConfig(containedctx.New()).
			WithSince("v1.44.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/sivchari/containedctx"),

		linter.NewConfig(contextcheck.New()).
			WithSince("v1.43.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/kkHAIKE/contextcheck"),

		linter.NewConfig(copyloopvar.New(&cfg.Linters.Settings.CopyLoopVar)).
			WithSince("v1.57.0").
			WithAutoFix().
			WithURL("https://github.com/karamaru-alpha/copyloopvar").
			WithNoopFallback(cfg, linter.IsGoLowerThanGo122()),

		linter.NewConfig(cyclop.New(&cfg.Linters.Settings.Cyclop)).
			WithSince("v1.37.0").
			WithURL("https://github.com/bkielbasa/cyclop"),

		linter.NewConfig(decorder.New(&cfg.Linters.Settings.Decorder)).
			WithSince("v1.44.0").
			WithURL("https://gitlab.com/bosi/decorder"),

		linter.NewConfig(depguard.New(&cfg.Linters.Settings.Depguard, placeholderReplacer)).
			WithSince("v1.4.0").
			WithURL("https://github.com/OpenPeeDeeP/depguard"),

		linter.NewConfig(dogsled.New(&cfg.Linters.Settings.Dogsled)).
			WithSince("v1.19.0").
			WithURL("https://github.com/alexkohler/dogsled"),

		linter.NewConfig(dupl.New(&cfg.Linters.Settings.Dupl)).
			WithSince("v1.0.0").
			WithURL("https://github.com/mibk/dupl"),

		linter.NewConfig(dupword.New(&cfg.Linters.Settings.DupWord)).
			WithSince("v1.50.0").
			WithAutoFix().
			WithURL("https://github.com/Abirdcfly/dupword"),

		linter.NewConfig(durationcheck.New()).
			WithSince("v1.37.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/charithe/durationcheck"),

		linter.NewConfig(embeddedstructfieldcheck.New(&cfg.Linters.Settings.EmbeddedStructFieldCheck)).
			WithSince("v2.2.0").
			WithURL("https://github.com/manuelarte/embeddedstructfieldcheck"),

		linter.NewConfig(errcheck.New(&cfg.Linters.Settings.Errcheck)).
			WithGroups(config.GroupStandard).
			WithSince("v1.0.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/kisielk/errcheck"),

		linter.NewConfig(errchkjson.New(&cfg.Linters.Settings.ErrChkJSON)).
			WithSince("v1.44.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/breml/errchkjson"),

		linter.NewConfig(errname.New()).
			WithSince("v1.42.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/Antonboom/errname"),

		linter.NewConfig(errorlint.New(&cfg.Linters.Settings.ErrorLint)).
			WithSince("v1.32.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://codeberg.org/polyfloyd/go-errorlint"),

		linter.NewConfig(exhaustive.New(&cfg.Linters.Settings.Exhaustive)).
			WithSince("v1.28.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/nishanths/exhaustive"),

		linter.NewConfig(exhaustruct.New(&cfg.Linters.Settings.Exhaustruct)).
			WithSince("v1.46.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/GaijinEntertainment/go-exhaustruct"),

		linter.NewConfig(exptostd.New()).
			WithSince("v1.63.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/ldez/exptostd"),

		linter.NewConfig(forbidigo.New(&cfg.Linters.Settings.Forbidigo)).
			WithSince("v1.34.0").
			// Strictly speaking,
			// the additional information is only needed when forbidigoCfg.AnalyzeTypes is chosen by the user.
			// But we don't know that here in all cases (sometimes config is not loaded),
			// so we have to assume that it is needed to be on the safe side.
			WithLoadForGoAnalysis().
			WithURL("https://github.com/ashanbrown/forbidigo"),

		linter.NewConfig(forcetypeassert.New()).
			WithSince("v1.38.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/gostaticanalysis/forcetypeassert"),

		linter.NewConfig(funcorder.New(&cfg.Linters.Settings.FuncOrder)).
			WithSince("v2.1.0").
			WithURL("https://github.com/manuelarte/funcorder"),

		linter.NewConfig(fatcontext.New(&cfg.Linters.Settings.Fatcontext)).
			WithSince("v1.58.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/Crocmagnon/fatcontext"),

		linter.NewConfig(funlen.New(&cfg.Linters.Settings.Funlen)).
			WithSince("v1.18.0").
			WithURL("https://github.com/ultraware/funlen"),

		linter.NewConfig(gci.New(&cfg.Linters.Settings.Gci)).
			WithSince("v1.30.0").
			WithAutoFix().
			WithURL("https://github.com/daixiang0/gci"),

		linter.NewConfig(ginkgolinter.New(&cfg.Linters.Settings.GinkgoLinter)).
			WithSince("v1.51.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/nunnatsa/ginkgolinter"),

		linter.NewConfig(gocheckcompilerdirectives.New()).
			WithSince("v1.51.0").
			WithURL("https://github.com/leighmcculloch/gocheckcompilerdirectives"),

		linter.NewConfig(gochecknoglobals.New()).
			WithSince("v1.12.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/leighmcculloch/gochecknoglobals"),

		linter.NewConfig(gochecknoinits.New()).
			WithSince("v1.12.0"),

		linter.NewConfig(gochecksumtype.New(&cfg.Linters.Settings.GoChecksumType)).
			WithSince("v1.55.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/alecthomas/go-check-sumtype"),

		linter.NewConfig(gocognit.New(&cfg.Linters.Settings.Gocognit)).
			WithSince("v1.20.0").
			WithURL("https://github.com/uudashr/gocognit"),

		linter.NewConfig(goconst.New(&cfg.Linters.Settings.Goconst)).
			WithSince("v1.0.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/jgautheron/goconst"),

		linter.NewConfig(gocritic.New(&cfg.Linters.Settings.Gocritic, placeholderReplacer)).
			WithSince("v1.12.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/go-critic/go-critic"),

		linter.NewConfig(gocyclo.New(&cfg.Linters.Settings.Gocyclo)).
			WithSince("v1.0.0").
			WithURL("https://github.com/fzipp/gocyclo"),

		linter.NewConfig(godoclint.New(&cfg.Linters.Settings.Godoclint)).
			WithSince("v2.5.0").
			WithURL("https://github.com/godoc-lint/godoc-lint"),

		linter.NewConfig(godot.New(&cfg.Linters.Settings.Godot)).
			WithSince("v1.25.0").
			WithAutoFix().
			WithURL("https://github.com/tetafro/godot"),

		linter.NewConfig(godox.New(&cfg.Linters.Settings.Godox)).
			WithSince("v1.19.0").
			WithURL("https://github.com/matoous/godox"),

		linter.NewConfig(err113.New()).
			WithSince("v1.26.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/Djarvur/go-err113"),

		linter.NewConfig(gofmt.New(&cfg.Linters.Settings.GoFmt)).
			WithSince("v1.0.0").
			WithAutoFix().
			WithURL("https://pkg.go.dev/cmd/gofmt"),

		linter.NewConfig(gofumpt.New(&cfg.Linters.Settings.GoFumpt)).
			WithSince("v1.28.0").
			WithAutoFix().
			WithURL("https://github.com/mvdan/gofumpt"),

		linter.NewConfig(golines.New(&cfg.Linters.Settings.GoLines)).
			WithSince("v2.0.0").
			WithAutoFix().
			WithURL("https://github.com/segmentio/golines"),

		linter.NewConfig(goheader.New(&cfg.Linters.Settings.Goheader, placeholderReplacer)).
			WithSince("v1.28.0").
			WithAutoFix().
			WithURL("https://github.com/denis-tingaikin/go-header"),

		linter.NewConfig(goimports.New(&cfg.Linters.Settings.GoImports)).
			WithSince("v1.20.0").
			WithAutoFix().
			WithURL("https://pkg.go.dev/golang.org/x/tools/cmd/goimports"),

		linter.NewConfig(mnd.New(&cfg.Linters.Settings.Mnd)).
			WithSince("v1.22.0").
			WithURL("https://github.com/tommy-muehle/go-mnd"),

		linter.NewConfig(modernize.New(&cfg.Linters.Settings.Modernize)).
			WithSince("v2.6.0").
			WithLoadForGoAnalysis().
			WithURL("https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize"),

		linter.NewConfig(gomoddirectives.New(&cfg.Linters.Settings.GoModDirectives)).
			WithSince("v1.39.0").
			WithURL("https://github.com/ldez/gomoddirectives"),

		linter.NewConfig(gomodguard.New(&cfg.Linters.Settings.Gomodguard)).
			WithSince("v1.25.0").
			DeprecatedWarning("new major version.", "v2.12.0",
				linter.Replacement("gomodguard_v2", gomodguard.Migration, &cfg.Linters.Settings.Gomodguard)).
			WithURL("https://github.com/ryancurrah/gomodguard"),

		linter.NewConfig(gomodguard.NewV2(&cfg.Linters.Settings.Gomodguardv2)).
			WithSince("v2.12.0").
			WithURL("https://github.com/ryancurrah/gomodguard"),

		linter.NewConfig(goprintffuncname.New()).
			WithSince("v1.23.0").
			WithURL("https://github.com/golangci/go-printf-func-name"),

		linter.NewConfig(gosec.New(&cfg.Linters.Settings.Gosec)).
			WithSince("v1.0.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/securego/gosec"),

		linter.NewConfig(gosmopolitan.New(&cfg.Linters.Settings.Gosmopolitan)).
			WithSince("v1.53.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/xen0n/gosmopolitan"),

		linter.NewConfig(govet.New(&cfg.Linters.Settings.Govet)).
			WithGroups(config.GroupStandard).
			WithSince("v1.0.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://pkg.go.dev/cmd/vet"),

		linter.NewConfig(grouper.New(&cfg.Linters.Settings.Grouper)).
			WithSince("v1.44.0").
			WithURL("https://github.com/leonklingele/grouper"),

		linter.NewConfig(iface.New(&cfg.Linters.Settings.Iface)).
			WithSince("v1.62.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/uudashr/iface"),

		linter.NewConfig(importas.New(&cfg.Linters.Settings.ImportAs)).
			WithSince("v1.38.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/julz/importas"),

		linter.NewConfig(inamedparam.New(&cfg.Linters.Settings.Inamedparam)).
			WithSince("v1.55.0").
			WithURL("https://github.com/macabu/inamedparam"),

		linter.NewConfig(ineffassign.New(&cfg.Linters.Settings.Ineffassign)).
			WithGroups(config.GroupStandard).
			WithSince("v1.0.0").
			WithURL("https://github.com/gordonklaus/ineffassign"),

		linter.NewConfig(interfacebloat.New(&cfg.Linters.Settings.InterfaceBloat)).
			WithSince("v1.49.0").
			WithURL("https://github.com/sashamelentyev/interfacebloat"),

		linter.NewConfig(intrange.New()).
			WithSince("v1.57.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/ckaznocha/intrange").
			WithNoopFallback(cfg, linter.IsGoLowerThanGo122()),

		linter.NewConfig(iotamixing.New(&cfg.Linters.Settings.IotaMixing)).
			WithSince("v2.5.0").
			WithURL("https://github.com/AdminBenni/iota-mixing"),

		linter.NewConfig(ireturn.New(&cfg.Linters.Settings.Ireturn)).
			WithSince("v1.43.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/butuzov/ireturn"),

		linter.NewConfig(lll.New(&cfg.Linters.Settings.Lll)).
			WithSince("v1.8.0"),

		linter.NewConfig(loggercheck.New(&cfg.Linters.Settings.LoggerCheck)).
			WithSince("v1.49.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/timonwong/loggercheck"),

		linter.NewConfig(maintidx.New(&cfg.Linters.Settings.MaintIdx)).
			WithSince("v1.44.0").
			WithURL("https://github.com/yagipy/maintidx"),

		linter.NewConfig(makezero.New(&cfg.Linters.Settings.Makezero)).
			WithSince("v1.34.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/ashanbrown/makezero"),

		linter.NewConfig(mirror.New()).
			WithSince("v1.53.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/butuzov/mirror"),

		linter.NewConfig(misspell.New(&cfg.Linters.Settings.Misspell)).
			WithSince("v1.8.0").
			WithAutoFix().
			WithURL("https://github.com/golangci/misspell"),

		linter.NewConfig(musttag.New(&cfg.Linters.Settings.MustTag)).
			WithSince("v1.51.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/go-simpler/musttag"),

		linter.NewConfig(nakedret.New(&cfg.Linters.Settings.Nakedret)).
			WithSince("v1.19.0").
			WithAutoFix().
			WithURL("https://github.com/alexkohler/nakedret"),

		linter.NewConfig(nestif.New(&cfg.Linters.Settings.Nestif)).
			WithSince("v1.25.0").
			WithURL("https://github.com/nakabonne/nestif"),

		linter.NewConfig(nilerr.New()).
			WithSince("v1.38.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/gostaticanalysis/nilerr"),

		linter.NewConfig(nilnesserr.New()).
			WithSince("v1.63.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/alingse/nilnesserr"),

		linter.NewConfig(nilnil.New(&cfg.Linters.Settings.NilNil)).
			WithSince("v1.43.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/Antonboom/nilnil"),

		linter.NewConfig(nlreturn.New(&cfg.Linters.Settings.Nlreturn)).
			WithSince("v1.30.0").
			WithAutoFix().
			WithURL("https://github.com/ssgreg/nlreturn"),

		linter.NewConfig(noctx.New()).
			WithSince("v1.28.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/sonatard/noctx"),

		linter.NewConfig(noinlineerr.New()).
			WithSince("v2.2.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/AlwxSin/noinlineerr"),

		linter.NewConfig(nonamedreturns.New(&cfg.Linters.Settings.NoNamedReturns)).
			WithSince("v1.46.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/firefart/nonamedreturns"),

		linter.NewConfig(nosprintfhostport.New()).
			WithSince("v1.46.0").
			WithURL("https://github.com/stbenjam/no-sprintf-host-port"),

		linter.NewConfig(paralleltest.New(&cfg.Linters.Settings.ParallelTest)).
			WithSince("v1.33.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/kunwardeep/paralleltest"),

		linter.NewConfig(perfsprint.New(&cfg.Linters.Settings.PerfSprint)).
			WithSince("v1.55.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/catenacyber/perfsprint"),

		linter.NewConfig(prealloc.New(&cfg.Linters.Settings.Prealloc)).
			WithSince("v1.19.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/alexkohler/prealloc"),

		linter.NewConfig(predeclared.New(&cfg.Linters.Settings.Predeclared)).
			WithSince("v1.35.0").
			WithURL("https://github.com/nishanths/predeclared"),

		linter.NewConfig(promlinter.New(&cfg.Linters.Settings.Promlinter)).
			WithSince("v1.40.0").
			WithURL("https://github.com/yeya24/promlinter"),

		linter.NewConfig(protogetter.New(&cfg.Linters.Settings.ProtoGetter)).
			WithSince("v1.55.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/ghostiam/protogetter"),

		linter.NewConfig(reassign.New(&cfg.Linters.Settings.Reassign)).
			WithSince("v1.49.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/curioswitch/go-reassign"),

		linter.NewConfig(recvcheck.New(&cfg.Linters.Settings.Recvcheck)).
			WithSince("v1.62.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/raeperd/recvcheck"),

		linter.NewConfig(revive.New(&cfg.Linters.Settings.Revive)).
			WithSince("v1.37.0").
			ConsiderSlow().
			WithAutoFix().
			WithURL("https://github.com/mgechev/revive"),

		linter.NewConfig(rowserrcheck.New(&cfg.Linters.Settings.RowsErrCheck)).
			WithSince("v1.23.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/golangci/rowserrcheck"),

		linter.NewConfig(sloglint.New(&cfg.Linters.Settings.Sloglint)).
			WithSince("v1.55.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/go-simpler/sloglint"),

		linter.NewConfig(sqlclosecheck.New()).
			WithSince("v1.28.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/ryanrolds/sqlclosecheck"),

		linter.NewConfig(spancheck.New(&cfg.Linters.Settings.Spancheck)).
			WithSince("v1.56.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/jjti/go-spancheck"),

		linter.NewConfig(staticcheck.New(&cfg.Linters.Settings.Staticcheck)).
			WithGroups(config.GroupStandard).
			WithSince("v1.0.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/dominikh/go-tools"),

		linter.NewConfig(swaggo.New()).
			WithSince("v2.2.0").
			WithAutoFix().
			WithURL("https://github.com/swaggo/swag"),

		linter.NewConfig(tagalign.New(&cfg.Linters.Settings.TagAlign)).
			WithSince("v1.53.0").
			WithAutoFix().
			WithURL("https://github.com/4meepo/tagalign"),

		linter.NewConfig(tagliatelle.New(&cfg.Linters.Settings.Tagliatelle)).
			WithSince("v1.40.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/ldez/tagliatelle"),

		linter.NewConfig(testableexamples.New()).
			WithSince("v1.50.0").
			WithURL("https://github.com/maratori/testableexamples"),

		linter.NewConfig(testifylint.New(&cfg.Linters.Settings.Testifylint)).
			WithSince("v1.55.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/Antonboom/testifylint"),

		linter.NewConfig(testpackage.New(&cfg.Linters.Settings.Testpackage)).
			WithSince("v1.25.0").
			WithURL("https://github.com/maratori/testpackage"),

		linter.NewConfig(thelper.New(&cfg.Linters.Settings.Thelper)).
			WithSince("v1.34.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/kulti/thelper"),

		linter.NewConfig(tparallel.New()).
			WithSince("v1.32.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/moricho/tparallel"),

		linter.NewConfig(golinters.NewTypecheck()).
			WithInternal().
			WithSince("v1.3.0"),

		linter.NewConfig(unconvert.New(&cfg.Linters.Settings.Unconvert)).
			WithSince("v1.0.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/mdempsky/unconvert"),

		linter.NewConfig(unparam.New(&cfg.Linters.Settings.Unparam)).
			WithSince("v1.9.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/mvdan/unparam"),

		linter.NewConfig(unqueryvet.New(&cfg.Linters.Settings.Unqueryvet)).
			WithSince("v2.5.0").
			WithURL("https://github.com/MirrexOne/unqueryvet"),

		linter.NewConfig(unused.New(&cfg.Linters.Settings.Unused)).
			WithGroups(config.GroupStandard).
			WithSince("v1.20.0").
			WithLoadForGoAnalysis().
			ConsiderSlow().
			WithChangeTypes().
			WithURL("https://github.com/dominikh/go-tools/tree/HEAD/unused"),

		linter.NewConfig(usestdlibvars.New(&cfg.Linters.Settings.UseStdlibVars)).
			WithSince("v1.48.0").
			WithAutoFix().
			WithURL("https://github.com/sashamelentyev/usestdlibvars"),

		linter.NewConfig(usetesting.New(&cfg.Linters.Settings.UseTesting)).
			WithSince("v1.63.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/ldez/usetesting"),

		linter.NewConfig(varnamelen.New(&cfg.Linters.Settings.Varnamelen)).
			WithSince("v1.43.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/blizzy78/varnamelen"),

		linter.NewConfig(wastedassign.New()).
			WithSince("v1.38.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/sanposhiho/wastedassign"),

		linter.NewConfig(whitespace.New(&cfg.Linters.Settings.Whitespace)).
			WithSince("v1.19.0").
			WithAutoFix().
			WithURL("https://github.com/ultraware/whitespace"),

		linter.NewConfig(wrapcheck.New(&cfg.Linters.Settings.Wrapcheck)).
			WithSince("v1.32.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/tomarrell/wrapcheck"),

		linter.NewConfig(wsl.NewV4(&cfg.Linters.Settings.WSL)).
			DeprecatedWarning("new major version.", "v2.2.0",
				linter.Replacement("wsl_v5", wsl.Migration, &cfg.Linters.Settings.WSL)).
			WithSince("v1.20.0").
			WithAutoFix().
			WithURL("https://github.com/bombsimon/wsl"),

		linter.NewConfig(wsl.NewV5(&cfg.Linters.Settings.WSLv5)).
			WithSince("v2.2.0").
			WithLoadForGoAnalysis().
			WithAutoFix().
			WithURL("https://github.com/bombsimon/wsl"),

		linter.NewConfig(zerologlint.New()).
			WithSince("v1.53.0").
			WithLoadForGoAnalysis().
			WithURL("https://github.com/ykadowak/zerologlint"),

		// nolintlint must be last because it looks at the results of all the previous linters for unused nolint directives
		linter.NewConfig(nolintlint.New(&cfg.Linters.Settings.NoLintLint)).
			WithSince("v1.26.0").
			WithAutoFix().
			WithURL("https://github.com/golangci/golangci-lint/tree/HEAD/pkg/golinters/nolintlint/internal"),
	}, nil
}
