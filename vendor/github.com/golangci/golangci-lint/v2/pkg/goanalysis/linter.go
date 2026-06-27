package goanalysis

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"strings"

	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

type LoadMode int

func (loadMode LoadMode) String() string {
	switch loadMode {
	case LoadModeNone:
		return "none"
	case LoadModeSyntax:
		return "syntax"
	case LoadModeTypesInfo:
		return "types info"
	case LoadModeWholeProgram:
		return "whole program"
	}
	panic(fmt.Sprintf("unknown load mode %d", loadMode))
}

const (
	LoadModeNone LoadMode = iota
	LoadModeSyntax
	LoadModeTypesInfo
	LoadModeWholeProgram
)

type Linter struct {
	name, desc              string
	analyzers               []*analysis.Analyzer
	cfg                     map[string]map[string]any
	issuesReporter          func(*linter.Context) []*Issue
	contextSetter           func(*linter.Context)
	loadMode                LoadMode
	needUseOriginalPackages bool
}

func NewLinter(name, desc string, analyzers []*analysis.Analyzer, cfg map[string]map[string]any) *Linter {
	return &Linter{name: name, desc: desc, analyzers: analyzers, cfg: cfg}
}

func NewLinterFromAnalyzer(analyzer *analysis.Analyzer) *Linter {
	return NewLinter(analyzer.Name, analyzer.Doc, []*analysis.Analyzer{analyzer}, nil)
}

func (lnt *Linter) Run(_ context.Context, lintCtx *linter.Context) ([]*result.Issue, error) {
	if err := lnt.preRun(lintCtx); err != nil {
		return nil, err
	}

	return runAnalyzers(lnt, lintCtx)
}

func (lnt *Linter) UseOriginalPackages() {
	lnt.needUseOriginalPackages = true
}

func (lnt *Linter) LoadMode() LoadMode {
	return lnt.loadMode
}

func (lnt *Linter) WithDesc(desc string) *Linter {
	lnt.desc = desc

	return lnt
}

func (lnt *Linter) WithVersion(v int) *Linter {
	if v == 0 {
		return lnt
	}

	for _, analyzer := range lnt.analyzers {
		if lnt.name != analyzer.Name {
			continue
		}

		// The analyzer name should be the same as the linter name to avoid displaying the name inside the reports.
		analyzer.Name = fmt.Sprintf("%s_v%d", analyzer.Name, v)
	}

	lnt.name = fmt.Sprintf("%s_v%d", lnt.name, v)

	return lnt
}

func (lnt *Linter) WithConfig(cfg map[string]any) *Linter {
	if len(cfg) == 0 {
		return lnt
	}

	lnt.cfg = map[string]map[string]any{
		lnt.name: cfg,
	}

	return lnt
}

func (lnt *Linter) WithLoadMode(loadMode LoadMode) *Linter {
	lnt.loadMode = loadMode
	return lnt
}

func (lnt *Linter) WithIssuesReporter(r func(*linter.Context) []*Issue) *Linter {
	lnt.issuesReporter = r
	return lnt
}

func (lnt *Linter) WithContextSetter(cs func(*linter.Context)) *Linter {
	lnt.contextSetter = cs
	return lnt
}

func (lnt *Linter) Name() string {
	return lnt.name
}

func (lnt *Linter) Desc() string {
	return lnt.desc
}

func (lnt *Linter) allAnalyzerNames() []string {
	var ret []string
	for _, a := range lnt.analyzers {
		ret = append(ret, a.Name)
	}
	return ret
}

func (*Linter) configureAnalyzer(a *analysis.Analyzer, cfg map[string]any) error {
	for k, v := range cfg {
		f := a.Flags.Lookup(k)
		if f == nil {
			validFlagNames := allFlagNames(&a.Flags)
			if len(validFlagNames) == 0 {
				return errors.New("analyzer doesn't have settings")
			}

			return fmt.Errorf("analyzer doesn't have setting %q, valid settings: %v",
				k, validFlagNames)
		}

		if err := f.Value.Set(valueToString(v)); err != nil {
			return fmt.Errorf("failed to set analyzer setting %q with value %q: %w", k, v, err)
		}
	}

	return nil
}

func (lnt *Linter) configure() error {
	analyzersMap := map[string]*analysis.Analyzer{}
	for _, a := range lnt.analyzers {
		analyzersMap[a.Name] = a
	}

	for analyzerName, analyzerSettings := range lnt.cfg {
		a := analyzersMap[analyzerName]
		if a == nil {
			return fmt.Errorf("settings key %q must be valid analyzer name, valid analyzers: %v",
				analyzerName, lnt.allAnalyzerNames())
		}

		if err := lnt.configureAnalyzer(a, analyzerSettings); err != nil {
			return fmt.Errorf("failed to configure analyzer %s: %w", analyzerName, err)
		}
	}

	return nil
}

func (lnt *Linter) preRun(lintCtx *linter.Context) error {
	if err := analysis.Validate(lnt.analyzers); err != nil {
		return fmt.Errorf("failed to validate analyzers: %w", err)
	}

	if err := lnt.configure(); err != nil {
		return fmt.Errorf("failed to configure analyzers: %w", err)
	}

	if lnt.contextSetter != nil {
		lnt.contextSetter(lintCtx)
	}

	return nil
}

func (lnt *Linter) getName() string {
	return lnt.name
}

func (lnt *Linter) getLinterNameForDiagnostic(*Diagnostic) string {
	return lnt.name
}

func (lnt *Linter) getAnalyzers() []*analysis.Analyzer {
	return lnt.analyzers
}

func (lnt *Linter) useOriginalPackages() bool {
	return lnt.needUseOriginalPackages
}

func (lnt *Linter) reportIssues(lintCtx *linter.Context) []*Issue {
	if lnt.issuesReporter != nil {
		return lnt.issuesReporter(lintCtx)
	}
	return nil
}

func (lnt *Linter) getLoadMode() LoadMode {
	return lnt.loadMode
}

func allFlagNames(fs *flag.FlagSet) []string {
	var ret []string
	fs.VisitAll(func(f *flag.Flag) {
		ret = append(ret, f.Name)
	})
	return ret
}

func valueToString(v any) string {
	if ss, ok := v.([]string); ok {
		return strings.Join(ss, ",")
	}

	if is, ok := v.([]any); ok {
		var ss []string
		for _, i := range is {
			ss = append(ss, fmt.Sprint(i))
		}

		return valueToString(ss)
	}

	return fmt.Sprint(v)
}

func DummyRun(_ *analysis.Pass) (any, error) {
	return nil, nil
}
