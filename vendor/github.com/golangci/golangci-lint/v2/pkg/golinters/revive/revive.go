package revive

import (
	"bytes"
	"cmp"
	"fmt"
	"go/token"
	"os"
	"reflect"
	"slices"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	hcversion "github.com/hashicorp/go-version"
	reviveConfig "github.com/mgechev/revive/config"
	"github.com/mgechev/revive/lint"
	"github.com/mgechev/revive/rule"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/internal"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/logutils"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const linterName = "revive"

var (
	debugf  = logutils.Debug(logutils.DebugKeyRevive)
	isDebug = logutils.HaveDebugTag(logutils.DebugKeyRevive)
)

func New(settings *config.ReviveSettings) *goanalysis.Linter {
	var mu sync.Mutex
	var resIssues []*goanalysis.Issue

	analyzer := &analysis.Analyzer{
		Name: linterName,
		Doc:  "Fast, configurable, extensible, flexible, and beautiful linter for Go. Drop-in replacement of golint.",
		Run:  goanalysis.DummyRun,
	}

	return goanalysis.
		NewLinterFromAnalyzer(analyzer).
		WithContextSetter(func(lintCtx *linter.Context) {
			w, err := newWrapper(settings)
			if err != nil {
				lintCtx.Log.Errorf("setup revive: %v", err)
				return
			}

			analyzer.Run = func(pass *analysis.Pass) (any, error) {
				issues, err := w.run(pass)
				if err != nil {
					return nil, err
				}

				if len(issues) == 0 {
					return nil, nil
				}

				mu.Lock()
				resIssues = append(resIssues, issues...)
				mu.Unlock()

				return nil, nil
			}
		}).
		WithIssuesReporter(func(*linter.Context) []*goanalysis.Issue {
			return resIssues
		}).
		WithLoadMode(goanalysis.LoadModeSyntax)
}

type wrapper struct {
	revive       lint.Linter
	lintingRules []lint.Rule
	conf         *lint.Config
}

func newWrapper(settings *config.ReviveSettings) (*wrapper, error) {
	conf, err := getConfig(settings)
	if err != nil {
		return nil, err
	}

	displayRules(conf)

	conf.GoVersion, err = hcversion.NewVersion(settings.Go)
	if err != nil {
		return nil, err
	}

	lintingRules, err := reviveConfig.GetLintingRules(conf, []lint.Rule{})
	if err != nil {
		return nil, err
	}

	return &wrapper{
		revive:       lint.New(os.ReadFile, settings.MaxOpenFiles),
		lintingRules: lintingRules,
		conf:         conf,
	}, nil
}

func (w *wrapper) run(pass *analysis.Pass) ([]*goanalysis.Issue, error) {
	packages := [][]string{internal.GetGoFileNames(pass)}

	failures, err := w.revive.Lint(packages, w.lintingRules, *w.conf)
	if err != nil {
		return nil, err
	}

	var issues []*goanalysis.Issue
	for failure := range failures {
		if failure.Confidence < w.conf.Confidence {
			continue
		}

		issues = append(issues, w.toIssue(pass, &failure))
	}

	return issues, nil
}

func (w *wrapper) toIssue(pass *analysis.Pass, failure *lint.Failure) *goanalysis.Issue {
	lineRangeTo := failure.Position.End.Line
	if failure.RuleName == (&rule.ExportedRule{}).Name() {
		lineRangeTo = failure.Position.Start.Line
	}

	issue := &result.Issue{
		Severity: string(severity(w.conf, failure)),
		Text:     fmt.Sprintf("%s: %s", failure.RuleName, failure.Failure),
		Pos: token.Position{
			Filename: failure.Position.Start.Filename,
			Line:     failure.Position.Start.Line,
			Offset:   failure.Position.Start.Offset,
			Column:   failure.Position.Start.Column,
		},
		LineRange: &result.Range{
			From: failure.Position.Start.Line,
			To:   lineRangeTo,
		},
		FromLinter: linterName,
	}

	if failure.ReplacementLine != "" {
		f := pass.Fset.File(token.Pos(failure.Position.Start.Offset))

		// Skip cgo files because the positions are wrong.
		if failure.Filename() == f.Name() {
			issue.SuggestedFixes = []analysis.SuggestedFix{{
				TextEdits: []analysis.TextEdit{{
					Pos: f.LineStart(failure.Position.Start.Line),
					End: goanalysis.EndOfLinePos(f, failure.Position.End.Line),
					// ReplacementLine doesn't contain the full line (missing newline), so we have to add a newline.
					// Also `failure.Position.End.Offset` is at the end of the node but not the line.
					NewText: []byte(failure.ReplacementLine + "\n"),
				}},
			}}
		}
	}

	return goanalysis.NewIssue(issue, pass)
}

// This function mimics the GetConfig function of revive.
// This allows to get default values and right types.
// https://github.com/golangci/golangci-lint/issues/1745
// https://github.com/mgechev/revive/blob/v1.13.0/config/config.go#L249
// https://github.com/mgechev/revive/blob/v1.13.0/config/config.go#L198-L204
func getConfig(cfg *config.ReviveSettings) (*lint.Config, error) {
	conf := defaultConfig()

	// Since the Go version is dynamic, this value must be neutralized in order to compare with a "zero value" of the configuration structure.
	zero := &config.ReviveSettings{Go: cfg.Go}

	if !reflect.DeepEqual(cfg, zero) {
		rawRoot := createConfigMap(cfg)
		buf := bytes.NewBuffer(nil)

		err := toml.NewEncoder(buf).Encode(rawRoot)
		if err != nil {
			return nil, fmt.Errorf("failed to encode configuration: %w", err)
		}

		conf = &lint.Config{}
		_, err = toml.NewDecoder(buf).Decode(conf)
		if err != nil {
			return nil, fmt.Errorf("failed to decode configuration: %w", err)
		}
	}

	normalizeConfig(conf)

	for k, r := range conf.Rules {
		err := r.Initialize()
		if err != nil {
			return nil, fmt.Errorf("error in config of rule %q: %w", k, err)
		}
		conf.Rules[k] = r
	}

	return conf, nil
}

func createConfigMap(cfg *config.ReviveSettings) map[string]any {
	const severity = "severity"

	rawRoot := map[string]any{
		"confidence":         cfg.Confidence,
		severity:             cfg.Severity,
		"errorCode":          cfg.ErrorCode,
		"warningCode":        cfg.WarningCode,
		"enableAllRules":     cfg.EnableAllRules,
		"enableDefaultRules": cfg.EnableDefaultRules,

		// Should be managed with `linters.exclusions.generated`.
		"ignoreGeneratedHeader": false,
	}

	rawDirectives := map[string]map[string]any{}
	for _, directive := range cfg.Directives {
		rawDirectives[directive.Name] = map[string]any{
			severity: directive.Severity,
		}
	}

	if len(rawDirectives) > 0 {
		rawRoot["directive"] = rawDirectives
	}

	rawRules := map[string]map[string]any{}
	for _, s := range cfg.Rules {
		rawRules[s.Name] = map[string]any{
			severity:    s.Severity,
			"arguments": safeTomlSlice(s.Arguments),
			"disabled":  s.Disabled,
			"exclude":   s.Exclude,
		}
	}

	if len(rawRules) > 0 {
		rawRoot["rule"] = rawRules
	}

	return rawRoot
}

func safeTomlSlice(r []any) []any {
	if len(r) == 0 {
		return nil
	}

	if _, ok := r[0].(map[any]any); !ok {
		return r
	}

	var typed []any
	for _, elt := range r {
		item := map[string]any{}
		for k, v := range elt.(map[any]any) {
			item[k.(string)] = v
		}

		typed = append(typed, item)
	}

	return typed
}

// This element is not exported by revive, so we need copy the code.
// Extracted from https://github.com/mgechev/revive/blob/v1.15.0/config/config.go#L16
var defaultRules = []lint.Rule{
	&rule.VarDeclarationsRule{},
	&rule.PackageCommentsRule{},
	&rule.DotImportsRule{},
	&rule.BlankImportsRule{},
	&rule.ExportedRule{},
	&rule.VarNamingRule{},
	&rule.IndentErrorFlowRule{},
	&rule.RangeRule{},
	&rule.ErrorfRule{},
	&rule.ErrorNamingRule{},
	&rule.ErrorStringsRule{},
	&rule.ReceiverNamingRule{},
	&rule.IncrementDecrementRule{},
	&rule.ErrorReturnRule{},
	&rule.UnexportedReturnRule{},
	&rule.TimeNamingRule{},
	&rule.ContextKeysType{},
	&rule.ContextAsArgumentRule{},
	&rule.EmptyBlockRule{},
	&rule.SuperfluousElseRule{},
	&rule.UnusedParamRule{},
	&rule.UnreachableCodeRule{},
	&rule.RedefinesBuiltinIDRule{},
}

var allRules = append([]lint.Rule{
	&rule.AddConstantRule{},
	&rule.ArgumentsLimitRule{},
	&rule.AtomicRule{},
	&rule.BannedCharsRule{},
	&rule.BareReturnRule{},
	&rule.BoolLiteralRule{},
	&rule.CallToGCRule{},
	&rule.CognitiveComplexityRule{},
	&rule.CommentsDensityRule{},
	&rule.CommentSpacingsRule{},
	&rule.ConfusingNamingRule{},
	&rule.ConfusingResultsRule{},
	&rule.ConstantLogicalExprRule{},
	&rule.CyclomaticRule{},
	&rule.DataRaceRule{},
	&rule.DeepExitRule{},
	&rule.DeferRule{},
	&rule.DuplicatedImportsRule{},
	&rule.EarlyReturnRule{},
	&rule.EmptyLinesRule{},
	&rule.EnforceMapStyleRule{},
	&rule.EnforceRepeatedArgTypeStyleRule{},
	&rule.EnforceSliceStyleRule{},
	&rule.EnforceSwitchStyleRule{},
	&rule.EpochNamingRule{},
	&rule.FileHeaderRule{},
	&rule.FileLengthLimitRule{},
	&rule.FilenameFormatRule{},
	&rule.FlagParamRule{},
	&rule.ForbiddenCallInWgGoRule{},
	&rule.FunctionLength{},
	&rule.FunctionResultsLimitRule{},
	&rule.GetReturnRule{},
	&rule.IdenticalBranchesRule{},
	&rule.IdenticalIfElseIfBranchesRule{},
	&rule.IdenticalIfElseIfConditionsRule{},
	&rule.IdenticalSwitchBranchesRule{},
	&rule.IdenticalSwitchConditionsRule{},
	&rule.IfReturnRule{},
	&rule.ImportAliasNamingRule{},
	&rule.ImportsBlocklistRule{},
	&rule.ImportShadowingRule{},
	&rule.InefficientMapLookupRule{},
	&rule.LineLengthLimitRule{},
	&rule.MaxControlNestingRule{},
	&rule.MaxPublicStructsRule{},
	&rule.ModifiesParamRule{},
	&rule.ModifiesValRecRule{},
	&rule.NestedStructs{},
	&rule.OptimizeOperandsOrderRule{},
	&rule.PackageDirectoryMismatchRule{},
	&rule.PackageNamingRule{},
	&rule.RangeValAddress{},
	&rule.RangeValInClosureRule{},
	&rule.RedundantBuildTagRule{},
	&rule.RedundantImportAlias{},
	&rule.RedundantTestMainExitRule{},
	&rule.StringFormatRule{},
	&rule.StringOfIntRule{},
	&rule.StructTagRule{},
	&rule.TimeDateRule{},
	&rule.TimeEqualRule{},
	&rule.UncheckedTypeAssertionRule{},
	&rule.UnconditionalRecursionRule{},
	&rule.UnexportedNamingRule{},
	&rule.UnhandledErrorRule{},
	&rule.UnnecessaryFormatRule{},
	&rule.UnnecessaryIfRule{},
	&rule.UnnecessaryStmtRule{},
	&rule.UnsecureURLSchemeRule{},
	&rule.UnusedReceiverRule{},
	&rule.UseAnyRule{},
	&rule.UseErrorsNewRule{},
	&rule.UseFmtPrintRule{},
	&rule.UselessBreak{},
	&rule.UselessFallthroughRule{},
	&rule.UseSlicesSort{},
	&rule.UseWaitGroupGoRule{},
	&rule.WaitGroupByValueRule{},
}, defaultRules...)

const defaultConfidence = 0.8

// This element is not exported by revive, so we need copy the code.
// Extracted from https://github.com/mgechev/revive/blob/v1.13.0/config/config.go#L209
func normalizeConfig(cfg *lint.Config) {
	// NOTE(ldez): this custom section for golangci-lint should be kept.
	// ---
	cfg.Confidence = cmp.Or(cfg.Confidence, defaultConfidence)
	cfg.Severity = cmp.Or(cfg.Severity, lint.SeverityWarning)
	// ---

	if len(cfg.Rules) == 0 {
		cfg.Rules = map[string]lint.RuleConfig{}
	}

	addRules := func(config *lint.Config, rules []lint.Rule) {
		for _, r := range rules {
			ruleName := r.Name()
			if _, ok := config.Rules[ruleName]; !ok {
				config.Rules[ruleName] = lint.RuleConfig{}
			}
		}
	}

	if cfg.EnableAllRules {
		addRules(cfg, allRules)
	} else if cfg.EnableDefaultRules {
		addRules(cfg, defaultRules)
	}

	severity := cfg.Severity
	if severity != "" {
		for k, v := range cfg.Rules {
			if v.Severity == "" {
				v.Severity = severity
			}
			cfg.Rules[k] = v
		}
		for k, v := range cfg.Directives {
			if v.Severity == "" {
				v.Severity = severity
			}
			cfg.Directives[k] = v
		}
	}
}

// This element is not exported by revive, so we need copy the code.
// Extracted from https://github.com/mgechev/revive/blob/v1.13.0/config/config.go#L280
func defaultConfig() *lint.Config {
	defaultConfig := lint.Config{
		Confidence: defaultConfidence,
		Severity:   lint.SeverityWarning,
		Rules:      map[string]lint.RuleConfig{},
	}
	for _, r := range defaultRules {
		defaultConfig.Rules[r.Name()] = lint.RuleConfig{}
	}
	return &defaultConfig
}

func displayRules(conf *lint.Config) {
	if !isDebug {
		return
	}

	var enabledRules []string
	for k, r := range conf.Rules {
		if !r.Disabled {
			enabledRules = append(enabledRules, k)
		}
	}

	slices.Sort(enabledRules)

	debugf("All available rules (%d): %s.", len(allRules), strings.Join(extractRulesName(allRules), ", "))
	debugf("Default rules (%d): %s.", len(defaultRules), strings.Join(extractRulesName(defaultRules), ", "))
	debugf("Enabled by config rules (%d): %s.", len(enabledRules), strings.Join(enabledRules, ", "))

	debugf("revive configuration: %#v", conf)
}

func extractRulesName(rules []lint.Rule) []string {
	var names []string

	for _, r := range rules {
		names = append(names, r.Name())
	}

	slices.Sort(names)

	return names
}

// Extracted from https://github.com/mgechev/revive/blob/v1.13.0/formatter/severity.go
// Modified to use pointers (related to hugeParam rule).
func severity(cfg *lint.Config, failure *lint.Failure) lint.Severity {
	if cfg, ok := cfg.Rules[failure.RuleName]; ok && cfg.Severity == lint.SeverityError {
		return lint.SeverityError
	}
	if cfg, ok := cfg.Directives[failure.RuleName]; ok && cfg.Severity == lint.SeverityError {
		return lint.SeverityError
	}
	return lint.SeverityWarning
}
