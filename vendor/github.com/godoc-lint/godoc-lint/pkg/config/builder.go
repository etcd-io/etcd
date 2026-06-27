package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/godoc-lint/godoc-lint/pkg/model"
	"github.com/godoc-lint/godoc-lint/pkg/util"
)

// ConfigBuilder implements a configuration builder.
type ConfigBuilder struct {
	baseDir  string
	override *model.ConfigOverride

	// baseDirPlainConfig holds the plain config for the base directory.
	//
	// This is meant to be used for integrating with umbrella linters (e.g.
	// golangci-lint) where the root config comes from a different
	// source/format.
	baseDirPlainConfig *PlainConfig
}

// NewConfigBuilder crates a new instance of the corresponding struct.
func NewConfigBuilder(baseDir string) *ConfigBuilder {
	return &ConfigBuilder{
		baseDir: baseDir,
	}
}

// WithBaseDirPlainConfig sets the plain config for the base directory. This is
// meant to be used for integrating with umbrella linters (e.g. golangci-lint)
// where the root config comes from a different source/format.
func (cb *ConfigBuilder) WithBaseDirPlainConfig(baseDirPlainConfig *PlainConfig) *ConfigBuilder {
	cb.baseDirPlainConfig = baseDirPlainConfig
	return cb
}

// GetConfig implements the corresponding interface method.
func (cb *ConfigBuilder) GetConfig(cwd string) (model.Config, error) {
	return cb.build(cwd)
}

func (cb *ConfigBuilder) resolvePlainConfig(cwd string) (*PlainConfig, *PlainConfig, string, string, error) {
	def := getDefaultPlainConfig()

	if !util.IsPathUnderBaseDir(cb.baseDir, cwd) {
		if pcfg, filePath, err := cb.resolvePlainConfigAtBaseDir(); err != nil {
			return nil, nil, "", "", err
		} else if pcfg != nil {
			return pcfg, def, cb.baseDir, filePath, nil
		}
		return def, def, cb.baseDir, "", nil
	}

	path := cwd
	for {
		rel, err := filepath.Rel(cb.baseDir, path)
		if err != nil {
			return nil, nil, "", "", err
		}

		if rel == "." {
			if pcfg, filePath, err := cb.resolvePlainConfigAtBaseDir(); err != nil {
				return nil, nil, "", "", err
			} else if pcfg != nil {
				return pcfg, def, cb.baseDir, filePath, nil
			}
			return def, def, cb.baseDir, "", nil
		}

		if pcfg, filePath, err := findConventionalConfigFile(path); err != nil {
			return nil, nil, "", "", err
		} else if pcfg != nil {
			return pcfg, def, path, filePath, nil
		}

		path = filepath.Dir(path)
	}
}

func (cb *ConfigBuilder) resolvePlainConfigAtBaseDir() (*PlainConfig, string, error) {
	// TODO(babakks): refactor this to a sync.OnceValue for performance

	if cb.override != nil && cb.override.ConfigFilePath != nil {
		pcfg, err := FromYAMLFile(*cb.override.ConfigFilePath)
		if err != nil {
			return nil, "", err
		}
		return pcfg, *cb.override.ConfigFilePath, nil
	}

	if pcfg, filePath, err := findConventionalConfigFile(cb.baseDir); err != nil {
		return nil, "", err
	} else if pcfg != nil {
		return pcfg, filePath, nil
	}

	if cb.baseDirPlainConfig != nil {
		return cb.baseDirPlainConfig, "", nil
	}
	return nil, "", nil
}

func findConventionalConfigFile(dir string) (*PlainConfig, string, error) {
	for _, dcf := range defaultConfigFiles {
		path := filepath.Join(dir, dcf)
		if fi, err := os.Stat(path); err != nil || fi.IsDir() {
			continue
		}
		pcfg, err := FromYAMLFile(path)
		if err != nil {
			return nil, "", err
		}
		return pcfg, path, nil
	}
	return nil, "", nil
}

// build creates the configuration struct.
//
// It checks a sequence of sources:
//  1. Custom config file path
//  2. Default configuration files (e.g., `.godoc-lint.yaml`)
//
// If none was available, the default configuration will be returned.
//
// The method also does the following:
//   - Applies override flags (e.g., enable, or disable).
//   - Validates the final configuration.
func (cb *ConfigBuilder) build(cwd string) (*config, error) {
	pcfg, def, configCWD, configFilePath, err := cb.resolvePlainConfig(cwd)
	if err != nil {
		return nil, err
	}

	if err := pcfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config at %q: %w", configFilePath, err)
	}

	toValidRuleSet := func(s []string) (*model.RuleSet, []string) {
		if s == nil {
			return nil, nil
		}
		invalids := make([]string, 0, len(s))
		rules := make([]model.Rule, 0, len(s))
		for _, v := range s {
			if !model.AllRules.Has(model.Rule(v)) {
				invalids = append(invalids, v)
				continue
			}
			rules = append(rules, model.Rule(v))
		}
		set := model.RuleSet{}.Add(rules...)
		return &set, invalids
	}

	toValidRegexpSlice := func(s []string) ([]*regexp.Regexp, []string) {
		if s == nil {
			return nil, nil
		}
		var invalids []string
		var regexps []*regexp.Regexp
		for _, v := range s {
			re, err := regexp.Compile(v)
			if err != nil {
				invalids = append(invalids, v)
				continue
			}
			regexps = append(regexps, re)
		}
		return regexps, invalids
	}

	var errs []error

	result := &config{
		cwd:            configCWD,
		configFilePath: configFilePath,
	}

	var enabledRules *model.RuleSet
	if cb.override != nil && cb.override.Enable != nil {
		enabledRules = cb.override.Enable
	} else {
		raw := pcfg.Enable
		if raw == nil {
			raw = def.Enable
		}
		rs, invalids := toValidRuleSet(raw)
		if len(invalids) > 0 {
			errs = append(errs, fmt.Errorf("invalid rule(s) name to enable: %q", invalids))
		} else {
			enabledRules = rs
		}
	}

	var disabledRules *model.RuleSet
	if cb.override != nil && cb.override.Disable != nil {
		disabledRules = cb.override.Disable
	} else {
		raw := pcfg.Disable
		if raw == nil {
			raw = def.Disable
		}
		rs, invalids := toValidRuleSet(raw)
		if len(invalids) > 0 {
			errs = append(errs, fmt.Errorf("invalid rule(s) name to disable: %q", invalids))
		} else {
			disabledRules = rs
		}
	}

	if cb.override != nil && cb.override.Include != nil {
		result.includeAsRegexp = cb.override.Include
	} else {
		raw := pcfg.Include
		if raw == nil {
			raw = def.Include
		}
		rs, invalids := toValidRegexpSlice(raw)
		if len(invalids) > 0 {
			errs = append(errs, fmt.Errorf("invalid path pattern(s) to include: %q", invalids))
		} else {
			result.includeAsRegexp = rs
		}
	}

	if cb.override != nil && cb.override.Exclude != nil {
		result.excludeAsRegexp = cb.override.Exclude
	} else {
		raw := pcfg.Exclude
		if raw == nil {
			raw = def.Exclude
		}
		rs, invalids := toValidRegexpSlice(raw)
		if len(invalids) > 0 {
			errs = append(errs, fmt.Errorf("invalid path pattern(s) to exclude: %q", invalids))
		} else {
			result.excludeAsRegexp = rs
		}
	}

	if cb.override != nil && cb.override.Default != nil {
		result.rulesToApply = model.DefaultSetToRules[*cb.override.Default]
	} else {
		raw := pcfg.Default
		if raw == nil {
			raw = def.Default // never nil
		}

		if !slices.Contains(model.DefaultSetValues, model.DefaultSet(*raw)) {
			errs = append(errs, fmt.Errorf("invalid default set %q; must be one of %q", *raw, model.DefaultSetValues))
		} else {
			result.rulesToApply = model.DefaultSetToRules[model.DefaultSet(*raw)]
		}
	}

	var maxLenIgnore []*regexp.Regexp
	rawMaxLenIgnore := def.Options.MaxLenIgnorePatterns
	if pcfg.Options != nil && pcfg.Options.MaxLenIgnorePatterns != nil {
		rawMaxLenIgnore = pcfg.Options.MaxLenIgnorePatterns
	}
	if len(rawMaxLenIgnore) > 0 {
		rs, invalids := toValidRegexpSlice(rawMaxLenIgnore)
		if len(invalids) > 0 {
			errs = append(errs, fmt.Errorf("invalid max-len ignore pattern(s): %q", invalids))
		} else {
			maxLenIgnore = rs
		}
	}

	if errs != nil {
		return nil, errors.Join(errs...)
	}

	if enabledRules != nil {
		result.rulesToApply = result.rulesToApply.Merge(*enabledRules)
	}
	if disabledRules != nil {
		result.rulesToApply = result.rulesToApply.Remove(disabledRules.List()...)
	}

	// To avoid being too strict, we don't complain if a rule is enabled and disabled at the same time.

	resolvedOptions := &model.RuleOptions{}
	transferPrimitiveOptions(resolvedOptions, def.Options) // def.Options is never nil
	if pcfg.Options != nil {
		transferPrimitiveOptions(resolvedOptions, pcfg.Options)
	}
	resolvedOptions.MaxLenIgnorePatterns = maxLenIgnore

	result.options = resolvedOptions

	return result, nil
}

// SetOverride implements the corresponding interface method.
func (cb *ConfigBuilder) SetOverride(override *model.ConfigOverride) {
	cb.override = override
}

func transferPrimitiveOptions(target *model.RuleOptions, source *PlainRuleOptions) {
	transferIfNotNil(&target.MaxLenLength, source.MaxLenLength)
	transferIfNotNil(&target.MaxLenIncludeTests, source.MaxLenIncludeTests)
	transferIfNotNil(&target.PkgDocIncludeTests, source.PkgDocIncludeTests)
	transferIfNotNil(&target.SinglePkgDocIncludeTests, source.SinglePkgDocIncludeTests)
	transferIfNotNil(&target.RequirePkgDocIncludeTests, source.RequirePkgDocIncludeTests)
	transferIfNotNil(&target.RequireDocIncludeTests, source.RequireDocIncludeTests)
	transferIfNotNil(&target.RequireDocIgnoreExported, source.RequireDocIgnoreExported)
	transferIfNotNil(&target.RequireDocIgnoreUnexported, source.RequireDocIgnoreUnexported)
	transferIfNotNil(&target.StartWithNameIncludeTests, source.StartWithNameIncludeTests)
	transferIfNotNil(&target.StartWithNameIncludeUnexported, source.StartWithNameIncludeUnexported)
	transferIfNotNil(&target.RequireStdlibDoclinkIncludeTests, source.RequireStdlibDoclinkIncludeTests)
	transferIfNotNil(&target.NoUnusedLinkIncludeTests, source.NoUnusedLinkIncludeTests)
}

func transferIfNotNil[T any](dst, src *T) {
	if src == nil {
		return
	}
	*dst = *src
}
