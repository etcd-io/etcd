package config

import (
	"path/filepath"
	"regexp"

	"github.com/godoc-lint/godoc-lint/pkg/model"
)

// config represents the godoc-lint analyzer configuration.
type config struct {
	// cwd holds the directory that the configuration is applied to. This is the
	// way to find out relative paths to include/exclude based on the config
	// file.
	cwd string

	// configFilePath holds the path to the configuration file. If there is no
	// configuration file, which is the case when the default is used, this will
	// be an empty string.
	configFilePath string

	includeAsRegexp []*regexp.Regexp
	excludeAsRegexp []*regexp.Regexp
	rulesToApply    model.RuleSet
	options         *model.RuleOptions
}

// GetConfigFilePath implements the corresponding interface method.
func (c *config) GetConfigFilePath() string {
	return c.configFilePath
}

// GetCWD implements the corresponding interface method.
func (c *config) GetCWD() string {
	return c.cwd
}

// IsAnyRuleApplicable implements the corresponding interface method.
func (c *config) IsAnyRuleApplicable(rs model.RuleSet) bool {
	return c.rulesToApply.HasCommonsWith(rs)
}

// IsPathApplicable implements the corresponding interface method.
func (c *config) IsPathApplicable(path string) bool {
	p, err := filepath.Rel(c.cwd, path)
	if err != nil {
		p = path
	}

	// To ensure a consistent behavior on different platform (with the same
	// configuration), we convert the path to a Unix-style path.
	asUnixPath := filepath.ToSlash(p)

	for _, re := range c.excludeAsRegexp {
		if re.MatchString(asUnixPath) {
			return false
		}
	}
	if c.includeAsRegexp == nil {
		return true
	}
	for _, re := range c.includeAsRegexp {
		if re.MatchString(asUnixPath) {
			return true
		}
	}
	return false
}

// GetRuleOptions implements the corresponding interface method.
func (c *config) GetRuleOptions() *model.RuleOptions {
	return c.options
}
