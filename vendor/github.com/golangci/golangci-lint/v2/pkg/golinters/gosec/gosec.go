package gosec

import (
	"fmt"
	"go/token"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"

	"github.com/securego/gosec/v2"
	"github.com/securego/gosec/v2/analyzers"
	"github.com/securego/gosec/v2/issue"
	"github.com/securego/gosec/v2/rules"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const linterName = "gosec"

func New(settings *config.GoSecSettings) *goanalysis.Linter {
	var mu sync.Mutex
	var resIssues []*goanalysis.Issue

	conf := gosec.NewConfig()

	var ruleFilters []rules.RuleFilter
	var analyzerFilters []analyzers.AnalyzerFilter
	if settings != nil {
		// TODO(ldez) to remove when the problem will be fixed by gosec.
		// https://github.com/securego/gosec/issues/1211
		// https://github.com/securego/gosec/issues/1209
		settings.Excludes = append(settings.Excludes, "G407")

		ruleFilters = createRuleFilters(settings.Includes, settings.Excludes)
		analyzerFilters = createAnalyzerFilters(settings.Includes, settings.Excludes)
		conf = toGosecConfig(settings)
	}

	logger := log.New(io.Discard, "", 0)

	ruleDefinitions := rules.Generate(false, ruleFilters...)
	analyzerDefinitions := analyzers.Generate(false, analyzerFilters...)

	analyzer := &analysis.Analyzer{
		Name: linterName,
		Doc:  "Inspects source code for security problems",
		Run:  goanalysis.DummyRun,
	}

	return goanalysis.
		NewLinterFromAnalyzer(analyzer).
		WithContextSetter(func(lintCtx *linter.Context) {
			analyzer.Run = func(pass *analysis.Pass) (any, error) {
				// The `gosecAnalyzer` is here because of concurrency issue.
				gosecAnalyzer := gosec.NewAnalyzer(conf, true, false, false, settings.Concurrency, logger)

				gosecAnalyzer.LoadRules(ruleDefinitions.RulesInfo())
				gosecAnalyzer.LoadAnalyzers(analyzerDefinitions.AnalyzersInfo())

				issues := runGoSec(lintCtx, pass, settings, gosecAnalyzer)

				mu.Lock()
				resIssues = append(resIssues, issues...)
				mu.Unlock()

				return nil, nil
			}
		}).
		WithIssuesReporter(func(*linter.Context) []*goanalysis.Issue {
			return resIssues
		}).
		WithLoadMode(goanalysis.LoadModeTypesInfo)
}

func runGoSec(lintCtx *linter.Context, pass *analysis.Pass, settings *config.GoSecSettings, analyzer *gosec.Analyzer) []*goanalysis.Issue {
	pkg := &packages.Package{
		Fset:      pass.Fset,
		Syntax:    pass.Files,
		Types:     pass.Pkg,
		TypesInfo: pass.TypesInfo,
	}

	analyzer.CheckRules(pkg)
	analyzer.CheckAnalyzers(pkg)

	secIssues, _, _ := analyzer.Report()
	if len(secIssues) == 0 {
		return nil
	}

	severity, err := convertToScore(settings.Severity)
	if err != nil {
		lintCtx.Log.Warnf("The provided severity %v", err)
	}

	confidence, err := convertToScore(settings.Confidence)
	if err != nil {
		lintCtx.Log.Warnf("The provided confidence %v", err)
	}

	secIssues = filterIssues(secIssues, severity, confidence)

	issues := make([]*goanalysis.Issue, 0, len(secIssues))
	for _, i := range secIssues {
		text := fmt.Sprintf("%s: %s", i.RuleID, i.What)

		var r *result.Range

		line, err := strconv.Atoi(i.Line)
		if err != nil {
			r = &result.Range{}
			if n, rerr := fmt.Sscanf(i.Line, "%d-%d", &r.From, &r.To); rerr != nil || n != 2 {
				lintCtx.Log.Warnf("Can't convert gosec line number %q of %v to int: %s", i.Line, i, err)
				continue
			}
			line = r.From
		}

		column, err := strconv.Atoi(i.Col)
		if err != nil {
			lintCtx.Log.Warnf("Can't convert gosec column number %q of %v to int: %s", i.Col, i, err)
			continue
		}

		issues = append(issues, goanalysis.NewIssue(&result.Issue{
			Severity: convertScoreToString(i.Severity),
			Pos: token.Position{
				Filename: i.File,
				Line:     line,
				Column:   column,
			},
			Text:       text,
			LineRange:  r,
			FromLinter: linterName,
		}, pass))
	}

	return issues
}

func toGosecConfig(settings *config.GoSecSettings) gosec.Config {
	conf := gosec.NewConfig()

	for k, v := range settings.Config {
		if k == gosec.Globals {
			convertGosecGlobals(v, conf)
			continue
		}

		// Uses ToUpper because the parsing of the map's key change the key to lowercase.
		// The value is not impacted by that: the case is respected.
		conf.Set(strings.ToUpper(k), v)
	}

	return conf
}

func convertScoreToString(score issue.Score) string {
	switch score {
	case issue.Low:
		return "low"
	case issue.Medium:
		return "medium"
	case issue.High:
		return "high"
	default:
		return ""
	}
}

// based on https://github.com/securego/gosec/blob/47bfd4eb6fc7395940933388550b547538b4c946/config.go#L52-L62
func convertGosecGlobals(globalOptionFromConfig any, conf gosec.Config) {
	globalOptionMap, ok := globalOptionFromConfig.(map[string]any)
	if !ok {
		return
	}

	for k, v := range globalOptionMap {
		option := gosec.GlobalOption(k)

		// Set nosec global option only if the value is true
		// https://github.com/securego/gosec/blob/v2.21.4/analyzer.go#L572
		if option == gosec.Nosec && v == false {
			continue
		}

		conf.SetGlobal(option, fmt.Sprintf("%v", v))
	}
}

// based on https://github.com/securego/gosec/blob/81cda2f91fbe1bf4735feb55febcae03e697a92b/cmd/gosec/main.go#L258-L275
func createAnalyzerFilters(includes, excludes []string) []analyzers.AnalyzerFilter {
	var filters []analyzers.AnalyzerFilter

	if len(includes) > 0 {
		filters = append(filters, analyzers.NewAnalyzerFilter(false, includes...))
	}

	if len(excludes) > 0 {
		filters = append(filters, analyzers.NewAnalyzerFilter(true, excludes...))
	}

	return filters
}

// based on https://github.com/securego/gosec/blob/569328eade2ccbad4ce2d0f21ee158ab5356a5cf/cmd/gosec/main.go#L170-L188
func createRuleFilters(includes, excludes []string) []rules.RuleFilter {
	var filters []rules.RuleFilter

	if len(includes) > 0 {
		filters = append(filters, rules.NewRuleFilter(false, includes...))
	}

	if len(excludes) > 0 {
		filters = append(filters, rules.NewRuleFilter(true, excludes...))
	}

	return filters
}

// code borrowed from https://github.com/securego/gosec/blob/69213955dacfd560562e780f723486ef1ca6d486/cmd/gosec/main.go#L250-L262
func convertToScore(str string) (issue.Score, error) {
	str = strings.ToLower(str)
	switch str {
	case "", "low":
		return issue.Low, nil
	case "medium":
		return issue.Medium, nil
	case "high":
		return issue.High, nil
	default:
		return issue.Low, fmt.Errorf("'%s' is invalid, use low instead. Valid options: low, medium, high", str)
	}
}

// code borrowed from https://github.com/securego/gosec/blob/69213955dacfd560562e780f723486ef1ca6d486/cmd/gosec/main.go#L264-L276
func filterIssues(issues []*issue.Issue, severity, confidence issue.Score) []*issue.Issue {
	res := make([]*issue.Issue, 0)

	for _, i := range issues {
		if i.Severity >= severity && i.Confidence >= confidence {
			res = append(res, i)
		}
	}

	return res
}
