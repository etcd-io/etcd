package model

import (
	"maps"
	"regexp"
	"slices"
)

// ConfigBuilder defines a configuration builder.
type ConfigBuilder interface {
	// SetOverride sets the configuration override.
	SetOverride(override *ConfigOverride)

	// GetConfig builds and returns the configuration object for the given path.
	GetConfig(cwd string) (Config, error)
}

// DefaultSet defines the default set of rules to enable.
type DefaultSet string

const (
	// DefaultSetAll enables all rules.
	DefaultSetAll DefaultSet = "all"
	// DefaultSetNone disables all rules.
	DefaultSetNone DefaultSet = "none"
	// DefaultSetBasic enables a basic set of rules.
	DefaultSetBasic DefaultSet = "basic"

	// DefaultDefaultSet is the default set of rules to enable.
	DefaultDefaultSet = DefaultSetBasic
)

// DefaultSetToRules maps default sets to the corresponding rule sets.
var DefaultSetToRules = map[DefaultSet]RuleSet{
	DefaultSetAll:  AllRules,
	DefaultSetNone: {},
	DefaultSetBasic: func() RuleSet {
		return RuleSet{}.Add(
			PkgDocRule,
			SinglePkgDocRule,
			StartWithNameRule,
			DeprecatedRule,
		)
	}(),
}

// DefaultSetValues holds the valid values for DefaultSet.
var DefaultSetValues = func() []DefaultSet {
	values := slices.Collect(maps.Keys(DefaultSetToRules))
	slices.Sort(values)
	return values
}()

// ConfigOverride represents a configuration override.
//
// Non-nil values (including empty slices) indicate that the corresponding field
// is overridden.
type ConfigOverride struct {
	// ConfigFilePath is the path to config file.
	ConfigFilePath *string

	// Include is the overridden list of regexp patterns matching the files that
	// the linter should include.
	Include []*regexp.Regexp

	// Exclude is the overridden list of regexp patterns matching the files that
	// the linter should exclude.
	Exclude []*regexp.Regexp

	// Default is the default set of rules to enable.
	Default *DefaultSet

	// Enable is the overridden list of rules to enable.
	Enable *RuleSet

	// Disable is the overridden list of rules to disable.
	Disable *RuleSet
}

// NewConfigOverride returns a new config override instance.
func NewConfigOverride() *ConfigOverride {
	return &ConfigOverride{}
}

// Config defines an analyzer configuration.
type Config interface {
	// GetCWD returns the directory that the configuration is applied to. This
	// is the base to compute relative paths to include/exclude files.
	GetCWD() string

	// GetConfigFilePath returns the path to the configuration file. If there is
	// no configuration file, which is the case when the default is used, this
	// will be an empty string.
	GetConfigFilePath() string

	// IsAnyRuleEnabled determines if any of the given rule names is among
	// enabled rules, or not among disabled rules.
	IsAnyRuleApplicable(RuleSet) bool

	// IsPathApplicable determines if the given path matches the included path
	// patterns, or does not match the excluded path patterns.
	IsPathApplicable(path string) bool

	// Returns the rule-specific options.
	//
	// It never returns a nil pointer.
	GetRuleOptions() *RuleOptions
}

// RuleOptions represents individual linter rule configurations.
type RuleOptions struct {
	MaxLenLength                     uint
	MaxLenIncludeTests               bool
	MaxLenIgnorePatterns             []*regexp.Regexp
	PkgDocIncludeTests               bool
	SinglePkgDocIncludeTests         bool
	RequirePkgDocIncludeTests        bool
	RequireDocIncludeTests           bool
	RequireDocIgnoreExported         bool
	RequireDocIgnoreUnexported       bool
	StartWithNameIncludeTests        bool
	StartWithNameIncludeUnexported   bool
	RequireStdlibDoclinkIncludeTests bool
	NoUnusedLinkIncludeTests         bool
}
