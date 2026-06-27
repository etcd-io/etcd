package gocritic

import (
	"errors"
	"fmt"
	"go/ast"
	"go/types"
	"runtime"
	"slices"
	"strings"
	"sync"

	"github.com/go-critic/go-critic/checkers"
	gocriticlinter "github.com/go-critic/go-critic/linter"
	_ "github.com/quasilyte/go-ruleguard/dsl"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
)

const linterName = "gocritic"

var (
	debugf  = logutils.Debug(logutils.DebugKeyGoCritic)
	isDebug = logutils.HaveDebugTag(logutils.DebugKeyGoCritic)
)

func New(settings *config.GoCriticSettings, replacer *strings.Replacer) *goanalysis.Linter {
	wrapper := &goCriticWrapper{
		sizes: types.SizesFor("gc", runtime.GOARCH),
	}

	return goanalysis.
		NewLinterFromAnalyzer(&analysis.Analyzer{
			Name: linterName,
			Doc: `Provides diagnostics that check for bugs, performance and style issues.
Extensible without recompilation through dynamic rules.
Dynamic rules are written declaratively with AST patterns, filters, report message and optional suggestion.`,
			Run: func(pass *analysis.Pass) (any, error) {
				err := wrapper.run(pass)
				if err != nil {
					return nil, err
				}

				return nil, nil
			},
		}).
		WithContextSetter(func(context *linter.Context) {
			wrapper.init(context.Log, settings, replacer)
		}).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}

type goCriticWrapper struct {
	settingsWrapper *settingsWrapper
	sizes           types.Sizes
	once            sync.Once
}

func (w *goCriticWrapper) init(logger logutils.Log, settings *config.GoCriticSettings, replacer *strings.Replacer) {
	if settings == nil {
		return
	}

	w.once.Do(func() {
		err := checkers.InitEmbeddedRules()
		if err != nil {
			logger.Fatalf("%s: %v: setting an explicit GOROOT can fix this problem", linterName, err)
		}
	})

	settingsWrapper := newSettingsWrapper(logger, settings, replacer)

	if err := settingsWrapper.Load(); err != nil {
		logger.Fatalf("%s: invalid settings: %s", linterName, err)
	}

	w.settingsWrapper = settingsWrapper
}

func (w *goCriticWrapper) run(pass *analysis.Pass) error {
	if w.settingsWrapper == nil {
		return errors.New("the settings wrapper is nil")
	}

	linterCtx := gocriticlinter.NewContext(pass.Fset, w.sizes)

	linterCtx.SetGoVersion(w.settingsWrapper.Go)

	enabledCheckers, err := w.buildEnabledCheckers(linterCtx)
	if err != nil {
		return err
	}

	linterCtx.SetPackageInfo(pass.TypesInfo, pass.Pkg)

	needFileInfo := slices.ContainsFunc(enabledCheckers, func(c *gocriticlinter.Checker) bool {
		// Related to https://github.com/go-critic/go-critic/blob/440ff466685b41e67d231a1c4a8f5e093374ee93/checkers/importShadow_checker.go#L23
		return strings.EqualFold(c.Info.Name, "importShadow")
	})

	for _, f := range pass.Files {
		if needFileInfo {
			// Related to https://github.com/go-critic/go-critic/blob/440ff466685b41e67d231a1c4a8f5e093374ee93/checkers/importShadow_checker.go#L23
			linterCtx.SetFileInfo(f.Name.Name, f)
		}

		runOnFile(pass, f, enabledCheckers)
	}

	return nil
}

func (w *goCriticWrapper) buildEnabledCheckers(linterCtx *gocriticlinter.Context) ([]*gocriticlinter.Checker, error) {
	allLowerCasedParams := w.settingsWrapper.GetLowerCasedParams()

	var enabledCheckers []*gocriticlinter.Checker
	for _, info := range gocriticlinter.GetCheckersInfo() {
		if !w.settingsWrapper.IsCheckEnabled(info.Name) {
			continue
		}

		err := w.settingsWrapper.setCheckerParams(info, allLowerCasedParams)
		if err != nil {
			return nil, err
		}

		c, err := gocriticlinter.NewChecker(linterCtx, info)
		if err != nil {
			return nil, err
		}

		enabledCheckers = append(enabledCheckers, c)
	}

	return enabledCheckers, nil
}

func runOnFile(pass *analysis.Pass, f *ast.File, checks []*gocriticlinter.Checker) {
	for _, c := range checks {
		// All checkers are expected to use *lint.Context
		// as read-only structure, so no copying is required.
		for _, warn := range c.Check(f) {
			diag := analysis.Diagnostic{
				Pos:      warn.Pos,
				Category: c.Info.Name,
				Message:  fmt.Sprintf("%s: %s", c.Info.Name, warn.Text),
			}

			if warn.HasQuickFix() {
				diag.SuggestedFixes = []analysis.SuggestedFix{{
					TextEdits: []analysis.TextEdit{{
						Pos:     warn.Suggestion.From,
						End:     warn.Suggestion.To,
						NewText: warn.Suggestion.Replacement,
					}},
				}}
			}

			pass.Report(diag)
		}
	}
}
