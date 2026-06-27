package gomodguard

import (
	"sync"

	"github.com/ryancurrah/gomodguard"
	"golang.org/x/tools/go/analysis"

	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/goanalysis"
	"github.com/golangci/golangci-lint/v2/pkg/golinters/internal"
	"github.com/golangci/golangci-lint/v2/pkg/lint/linter"
	"github.com/golangci/golangci-lint/v2/pkg/result"
)

const linterName = "gomodguard"

func New(settings *config.GoModGuardSettings) *goanalysis.Linter {
	var issues []*goanalysis.Issue
	var mu sync.Mutex

	processorCfg := &gomodguard.Configuration{}
	if settings != nil {
		processorCfg.Allowed.Modules = settings.Allowed.Modules
		processorCfg.Allowed.Domains = settings.Allowed.Domains
		processorCfg.Blocked.LocalReplaceDirectives = settings.Blocked.LocalReplaceDirectives

		for n := range settings.Blocked.Modules {
			for k, v := range settings.Blocked.Modules[n] {
				m := map[string]gomodguard.BlockedModule{k: {
					Recommendations: v.Recommendations,
					Reason:          v.Reason,
				}}
				processorCfg.Blocked.Modules = append(processorCfg.Blocked.Modules, m)
				break
			}
		}

		for n := range settings.Blocked.Versions {
			for k, v := range settings.Blocked.Versions[n] {
				m := map[string]gomodguard.BlockedVersion{k: {
					Version: v.Version,
					Reason:  v.Reason,
				}}
				processorCfg.Blocked.Versions = append(processorCfg.Blocked.Versions, m)
				break
			}
		}
	}

	analyzer := &analysis.Analyzer{
		Name: linterName,
		Doc: "Allow and blocklist linter for direct Go module dependencies. " +
			"This is different from depguard where there are different block " +
			"types for example version constraints and module recommendations.",
		Run: goanalysis.DummyRun,
	}

	return goanalysis.NewLinterFromAnalyzer(analyzer).
		WithContextSetter(func(lintCtx *linter.Context) {
			processor, err := gomodguard.NewProcessor(processorCfg)
			if err != nil {
				lintCtx.Log.Warnf("running gomodguard failed: %s: if you are not using go modules "+
					"it is suggested to disable this linter", err)
				return
			}

			analyzer.Run = func(pass *analysis.Pass) (any, error) {
				gomodguardIssues := processor.ProcessFiles(internal.GetGoFileNames(pass))

				mu.Lock()
				defer mu.Unlock()

				for _, gomodguardIssue := range gomodguardIssues {
					issues = append(issues, goanalysis.NewIssue(&result.Issue{
						FromLinter: linterName,
						Pos:        gomodguardIssue.Position,
						Text:       gomodguardIssue.Reason,
					}, pass))
				}

				return nil, nil
			}
		}).
		WithIssuesReporter(func(*linter.Context) []*goanalysis.Issue {
			return issues
		}).
		WithLoadMode(goanalysis.LoadModeSyntax)
}

// Only used the set YAML struct tags.
type v2YAML struct {
	Allowed                []goModGuardv2Base    `yaml:"allowed,omitempty"`
	Blocked                []goModGuardv2Blocked `yaml:"blocked,omitempty"`
	LocalReplaceDirectives bool                  `yaml:"local-replace-directives,omitempty"`
}

// Only used the set YAML struct tags.
type goModGuardv2Base struct {
	Module    string `yaml:"module,omitempty"`
	Version   string `yaml:"version,omitempty"`
	MatchType string `yaml:"match-type,omitempty"`
}

// Only used the set YAML struct tags.
type goModGuardv2Blocked struct {
	goModGuardv2Base `yaml:",inline"`

	Recommendations []string `yaml:"recommendations,omitempty"`
	Reason          string   `yaml:"reason,omitempty"`
}

func Migration(old *config.GoModGuardSettings) any {
	if old == nil {
		return nil
	}

	if len(old.Allowed.Modules) == 0 && len(old.Allowed.Domains) == 0 && len(old.Blocked.Modules) == 0 && !old.Blocked.LocalReplaceDirectives {
		return nil
	}

	cfg := &v2YAML{
		LocalReplaceDirectives: old.Blocked.LocalReplaceDirectives,
	}

	for _, module := range old.Allowed.Modules {
		cfg.Allowed = append(cfg.Allowed, goModGuardv2Base{
			Module: module,
		})
	}

	for _, domain := range old.Allowed.Domains {
		cfg.Allowed = append(cfg.Allowed, goModGuardv2Base{
			Module:    domain + "/.*",
			MatchType: "regex",
		})
	}

	for _, blocked := range old.Blocked.Modules {
		for name, module := range blocked {
			cfg.Blocked = append(cfg.Blocked, goModGuardv2Blocked{
				goModGuardv2Base: goModGuardv2Base{
					Module: name,
				},
				Recommendations: module.Recommendations,
				Reason:          module.Reason,
			})
		}
	}

	return cfg
}
