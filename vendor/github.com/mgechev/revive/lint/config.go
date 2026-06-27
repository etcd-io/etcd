package lint

import (
	goversion "github.com/hashicorp/go-version"
)

// Arguments is type used for the arguments of a rule.
type Arguments = []any

// FileFilters is type used for modeling file filters to apply to rules.
type FileFilters = []*FileFilter

// RuleConfig is type used for the rule configuration.
type RuleConfig struct {
	Arguments Arguments
	Severity  Severity
	Disabled  bool
	// Exclude is rule-level file excludes, TOML related (strings).
	Exclude []string
	// excludeFilters is regex-based file filters, initialized from Exclude.
	excludeFilters []*FileFilter
}

// Initialize should be called after reading from TOML file.
func (rc *RuleConfig) Initialize() error {
	for _, f := range rc.Exclude {
		ff, err := ParseFileFilter(f)
		if err != nil {
			return err
		}
		rc.excludeFilters = append(rc.excludeFilters, ff)
	}
	return nil
}

// RulesConfig defines the config for all rules.
type RulesConfig = map[string]RuleConfig

// MustExclude checks if given filename `name` must be excluded.
func (rc *RuleConfig) MustExclude(name string) bool {
	for _, exclude := range rc.excludeFilters {
		if exclude.MatchFileName(name) {
			return true
		}
	}
	return false
}

// DirectiveConfig is type used for the linter directive configuration.
type DirectiveConfig struct {
	Severity Severity
}

// DirectivesConfig defines the config for all directives.
type DirectivesConfig = map[string]DirectiveConfig

// Config defines the config of the linter.
type Config struct {
	IgnoreGeneratedHeader bool             `toml:"ignoreGeneratedHeader"`
	Confidence            float64          `toml:"confidence"`
	Severity              Severity         `toml:"severity"`
	EnableAllRules        bool             `toml:"enableAllRules"`
	EnableDefaultRules    bool             `toml:"enableDefaultRules"`
	Rules                 RulesConfig      `toml:"rule"`
	ErrorCode             int              `toml:"errorCode"`
	WarningCode           int              `toml:"warningCode"`
	Directives            DirectivesConfig `toml:"directive"`
	Exclude               []string         `toml:"exclude"`
	// If set, overrides the go language version specified in go.mod of
	// packages being linted, and assumes this specific language version.
	GoVersion *goversion.Version `toml:"goVersion"`
}
