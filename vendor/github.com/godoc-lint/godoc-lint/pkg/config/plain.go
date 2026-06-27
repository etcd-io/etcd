package config

import (
	"errors"
	"fmt"
	"regexp"
	"slices"

	"github.com/godoc-lint/godoc-lint/pkg/model"
)

// PlainConfig represents the plain configuration type as users would provide
// via a config file (e.g., a YAML file).
type PlainConfig struct {
	Version *string           `yaml:"version" mapstructure:"version"`
	Exclude []string          `yaml:"exclude" mapstructure:"exclude"`
	Include []string          `yaml:"include" mapstructure:"include"`
	Default *string           `yaml:"default" mapstructure:"default"`
	Enable  []string          `yaml:"enable" mapstructure:"enable"`
	Disable []string          `yaml:"disable" mapstructure:"disable"`
	Options *PlainRuleOptions `yaml:"options" mapstructure:"options"`
}

// PlainRuleOptions represents the plain rule options as users would provide via
// a config file (e.g., a YAML file).
type PlainRuleOptions struct {
	MaxLenLength                     *uint    `yaml:"max-len/length" mapstructure:"max-len/length"`
	MaxLenIncludeTests               *bool    `yaml:"max-len/include-tests" mapstructure:"max-len/include-tests"`
	MaxLenIgnorePatterns             []string `yaml:"max-len/ignore-patterns" mapstructure:"max-len/ignore-patterns"`
	PkgDocIncludeTests               *bool    `yaml:"pkg-doc/include-tests" mapstructure:"pkg-doc/include-tests"`
	SinglePkgDocIncludeTests         *bool    `yaml:"single-pkg-doc/include-tests" mapstructure:"single-pkg-doc/include-tests"`
	RequirePkgDocIncludeTests        *bool    `yaml:"require-pkg-doc/include-tests" mapstructure:"require-pkg-doc/include-tests"`
	RequireDocIncludeTests           *bool    `yaml:"require-doc/include-tests" mapstructure:"require-doc/include-tests"`
	RequireDocIgnoreExported         *bool    `yaml:"require-doc/ignore-exported" mapstructure:"require-doc/ignore-exported"`
	RequireDocIgnoreUnexported       *bool    `yaml:"require-doc/ignore-unexported" mapstructure:"require-doc/ignore-unexported"`
	StartWithNameIncludeTests        *bool    `yaml:"start-with-name/include-tests" mapstructure:"start-with-name/include-tests"`
	StartWithNameIncludeUnexported   *bool    `yaml:"start-with-name/include-unexported" mapstructure:"start-with-name/include-unexported"`
	RequireStdlibDoclinkIncludeTests *bool    `yaml:"require-stdlib-doclink/include-tests" mapstructure:"require-stdlib-doclink/include-tests"`
	NoUnusedLinkIncludeTests         *bool    `yaml:"no-unused-link/include-tests" mapstructure:"no-unused-link/include-tests"`
}

// Validate validates the plain configuration.
func (pcfg *PlainConfig) Validate() error {
	var errs []error

	if pcfg.Default != nil && !slices.Contains(model.DefaultSetValues, model.DefaultSet(*pcfg.Default)) {
		errs = append(errs, fmt.Errorf("invalid default set %q; must be one of %q", *pcfg.Default, model.DefaultSetValues))
	}

	if invalids := getInvalidRules(pcfg.Enable); len(invalids) > 0 {
		errs = append(errs, fmt.Errorf("invalid rule name(s) to enable: %q", invalids))
	}

	if invalids := getInvalidRules(pcfg.Disable); len(invalids) > 0 {
		errs = append(errs, fmt.Errorf("invalid rule name(s) to disable: %q", invalids))
	}

	// To avoid being too strict, we don't complain if a rule is enabled and disabled at the same time.

	if invalids := getInvalidRegexps(pcfg.Include); len(invalids) > 0 {
		errs = append(errs, fmt.Errorf("invalid inclusion pattern(s): %q", invalids))
	}

	if invalids := getInvalidRegexps(pcfg.Exclude); len(invalids) > 0 {
		errs = append(errs, fmt.Errorf("invalid exclusion pattern(s): %q", invalids))
	}

	if pcfg.Options != nil {
		if invalids := getInvalidRegexps(pcfg.Options.MaxLenIgnorePatterns); len(invalids) > 0 {
			errs = append(errs, fmt.Errorf("invalid max-len ignore pattern(s): %q", invalids))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func getInvalidRules(names []string) []string {
	invalids := make([]string, 0, len(names))
	for _, element := range names {
		if !model.AllRules.Has(model.Rule(element)) {
			invalids = append(invalids, element)
		}
	}
	return invalids
}

func getInvalidRegexps(values []string) []string {
	invalids := make([]string, 0, len(values))
	for _, element := range values {
		if _, err := regexp.Compile(element); err != nil {
			invalids = append(invalids, element)
		}
	}
	return invalids
}
