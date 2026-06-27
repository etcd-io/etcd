// Package analysis provides the main analyzer implementation.
package analysis

import (
	"errors"
	"fmt"
	"path/filepath"

	"golang.org/x/tools/go/analysis"

	"github.com/godoc-lint/godoc-lint/pkg/model"
	"github.com/godoc-lint/godoc-lint/pkg/util"
)

const (
	metaName = "godoclint"
	metaDoc  = "Checks Golang's documentation practice (godoc)"
	metaURL  = "https://github.com/godoc-lint/godoc-lint"
)

// Analyzer implements the godoc-lint analyzer.
type Analyzer struct {
	baseDir   string
	cb        model.ConfigBuilder
	inspector model.Inspector
	reg       model.Registry
	exitFunc  func(int, error)

	analyzer *analysis.Analyzer
}

// NewAnalyzer returns a new instance of the corresponding analyzer.
func NewAnalyzer(baseDir string, cb model.ConfigBuilder, reg model.Registry, inspector model.Inspector, exitFunc func(int, error)) *Analyzer {
	result := &Analyzer{
		baseDir:   baseDir,
		cb:        cb,
		reg:       reg,
		inspector: inspector,
		exitFunc:  exitFunc,
		analyzer: &analysis.Analyzer{
			Name:     metaName,
			Doc:      metaDoc,
			URL:      metaURL,
			Requires: []*analysis.Analyzer{inspector.GetAnalyzer()},
		},
	}

	result.analyzer.Run = result.run
	return result
}

// GetAnalyzer returns the underlying analyzer.
func (a *Analyzer) GetAnalyzer() *analysis.Analyzer {
	return a.analyzer
}

func (a *Analyzer) run(pass *analysis.Pass) (any, error) {
	if len(pass.Files) == 0 {
		return nil, nil
	}

	ft := util.GetPassFileToken(pass.Files[0], pass)
	if ft == nil {
		err := errors.New("cannot prepare config")
		if a.exitFunc != nil {
			a.exitFunc(2, err)
		}
		return nil, err
	}

	if !util.IsPathUnderBaseDir(a.baseDir, ft.Name()) {
		return nil, nil
	}

	pkgDir := filepath.Dir(ft.Name())
	cfg, err := a.cb.GetConfig(pkgDir)
	if err != nil {
		err := fmt.Errorf("cannot prepare config: %w", err)
		if a.exitFunc != nil {
			a.exitFunc(2, err)
		}
		return nil, err
	}

	ir := pass.ResultOf[a.inspector.GetAnalyzer()].(*model.InspectorResult)
	if ir == nil || ir.Files == nil {
		return nil, nil
	}

	actx := &model.AnalysisContext{
		Config:          cfg,
		InspectorResult: ir,
		Pass:            pass,
	}

	for _, checker := range a.reg.List() {
		// TODO(babakks): This can be done once to improve performance.
		ruleSet := checker.GetCoveredRules()
		if !actx.Config.IsAnyRuleApplicable(ruleSet) {
			continue
		}

		if err := checker.Apply(actx); err != nil {
			return nil, fmt.Errorf("checker error: %w", err)
		}
	}
	return nil, nil
}
