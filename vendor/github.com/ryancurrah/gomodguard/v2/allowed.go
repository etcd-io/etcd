package gomodguard

import (
	"fmt"

	"github.com/Masterminds/semver/v3"
)

// Allowed is a list of modules that are allowed to be used.
type Allowed []AllowedModule

// AllowedModule is a single entry in the allowed list.
type AllowedModule struct {
	Module    string              `yaml:"module"`
	MatchType MatchType           `yaml:"match-type"`
	Version   *semver.Constraints `yaml:"version"`
	Matcher   Matcher             `yaml:"-"`
}

// CheckVersion returns true if the module version matches the allowed constraint,
// or if no version constraint is specified.
func (r *AllowedModule) CheckVersion(moduleVersion string) (bool, error) {
	if r.Version == nil {
		return true, nil
	}

	version, err := semver.NewVersion(moduleVersion)
	if err != nil {
		return false, err
	}

	return r.Version.Check(version), nil
}

// NotAllowedReason returns the reason why the module version is not allowed.
func (r *AllowedModule) NotAllowedReason(moduleVersion string) string {
	if r == nil || r.Version == nil {
		return "the module is not in the allowed modules list."
	}

	return fmt.Sprintf("version `%s` does not meet the allowed version constraint `%s`.", moduleVersion, r.Version)
}
