package migrate

import (
	"slices"

	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/ptr"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/versionone"
	"github.com/golangci/golangci-lint/v2/pkg/commands/internal/migrate/versiontwo"
	"github.com/golangci/golangci-lint/v2/pkg/config"
	"github.com/golangci/golangci-lint/v2/pkg/result/processors"
)

func toExclusions(old *versionone.Config) versiontwo.LinterExclusions {
	return versiontwo.LinterExclusions{
		Generated: toExclusionGenerated(old.Issues.ExcludeGenerated),
		Presets:   toPresets(old.Issues),
		Rules:     toExclusionRules(old),
		Paths:     toExclusionPaths(old.Issues),
	}
}

func toExclusionGenerated(excludeGenerated *string) *string {
	if excludeGenerated == nil || ptr.Deref(excludeGenerated) == "" {
		return ptr.Pointer("lax")
	}

	if ptr.Deref(excludeGenerated) == "strict" {
		return nil
	}

	return excludeGenerated
}

func toPresets(old versionone.Issues) []string {
	if old.UseDefaultExcludes != nil && !ptr.Deref(old.UseDefaultExcludes) {
		return nil
	}

	if len(old.IncludeDefaultExcludes) != 0 {
		var pp []string
		for p, rules := range processors.LinterExclusionPresets {
			found := slices.ContainsFunc(rules, func(rule config.ExcludeRule) bool {
				return slices.Contains(old.IncludeDefaultExcludes, rule.InternalReference)
			})
			if !found {
				pp = append(pp, p)
			}
		}

		slices.Sort(pp)

		return pp
	}

	return []string{
		config.ExclusionPresetComments,
		config.ExclusionPresetCommonFalsePositives,
		config.ExclusionPresetLegacy,
		config.ExclusionPresetStdErrorHandling,
	}
}

func toExclusionRules(old *versionone.Config) []versiontwo.ExcludeRule {
	var results []versiontwo.ExcludeRule

	for _, rule := range old.Issues.ExcludeRules {
		names := onlyLinterNames(convertStaticcheckLinterNames(convertAlternativeNames(rule.Linters)))
		if len(rule.Linters) > 0 && len(names) == 0 {
			continue
		}

		results = append(results, versiontwo.ExcludeRule{
			BaseRule: versiontwo.BaseRule{
				Linters:    names,
				Path:       rule.Path,
				PathExcept: rule.PathExcept,
				Text:       addPrefix(old.Issues, rule.Text),
				Source:     addPrefix(old.Issues, rule.Source),
			},
		})
	}

	for _, pattern := range old.Issues.ExcludePatterns {
		results = append(results, versiontwo.ExcludeRule{
			BaseRule: versiontwo.BaseRule{
				Path: ptr.Pointer(`(.+)\.go$`),
				Text: addPrefix(old.Issues, ptr.Pointer(pattern)),
			},
		})
	}

	return slices.Concat(results, linterTestExclusions(old.LintersSettings))
}

func addPrefix(old versionone.Issues, s *string) *string {
	if s == nil || ptr.Deref(s) == "" {
		return s
	}

	var prefix string
	if ptr.Deref(old.ExcludeCaseSensitive) {
		prefix = "(?i)"
	}

	return ptr.Pointer(prefix + ptr.Deref(s))
}

func linterTestExclusions(old versionone.LintersSettings) []versiontwo.ExcludeRule {
	var excludedTestLinters []string

	if ptr.Deref(old.Asasalint.IgnoreTest) {
		excludedTestLinters = append(excludedTestLinters, "asasalint")
	}
	if ptr.Deref(old.Cyclop.SkipTests) {
		excludedTestLinters = append(excludedTestLinters, "cyclop")
	}
	if ptr.Deref(old.Goconst.IgnoreTests) {
		excludedTestLinters = append(excludedTestLinters, "goconst")
	}
	if ptr.Deref(old.Gosmopolitan.IgnoreTests) {
		excludedTestLinters = append(excludedTestLinters, "gosmopolitan")
	}

	if len(excludedTestLinters) == 0 {
		return nil
	}

	return []versiontwo.ExcludeRule{{
		BaseRule: versiontwo.BaseRule{
			Linters: excludedTestLinters,
			Path:    ptr.Pointer(`(.+)_test\.go`),
		},
	}}
}

func toExclusionPaths(old versionone.Issues) []string {
	results := slices.Concat(old.ExcludeFiles, old.ExcludeDirs)

	if old.UseDefaultExcludeDirs == nil || ptr.Deref(old.UseDefaultExcludeDirs) {
		results = append(results, "third_party$", "builtin$", "examples$")
	}

	return results
}
