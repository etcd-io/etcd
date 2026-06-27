package gomodguard

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Blocked is a list of modules that are blocked and not to be used.
type Blocked []BlockedModule

// BlockedModule is a single entry in the blocked list.
type BlockedModule struct {
	Module          string              `yaml:"module"`
	MatchType       MatchType           `yaml:"match-type"`
	Recommendations []string            `yaml:"recommendations"`
	Reason          string              `yaml:"reason"`
	Version         *semver.Constraints `yaml:"version"`
	Matcher         Matcher             `yaml:"-"`
}

// CheckVersion returns true if the module version matches the blocked constraint.
// If no version constraint is specified, all versions are considered blocked.
func (r *BlockedModule) CheckVersion(moduleVersion string) (bool, error) {
	if r.Version == nil {
		return true, nil
	}

	version, err := semver.NewVersion(moduleVersion)
	if err != nil {
		return true, err
	}

	return r.Version.Check(version), nil
}

// BlockReason returns the reason why the module or version is blocked.
func (r *BlockedModule) BlockReason(currentModuleVersion string) string {
	var sb strings.Builder

	if r.Version != nil {
		_, _ = fmt.Fprintf(&sb, "version `%s` is blocked because it does not meet the version constraint `%s`.",
			currentModuleVersion, r.Version)
	}

	if len(r.Recommendations) > 0 {
		if sb.Len() > 0 {
			sb.WriteString(" ")
		}

		for i := range r.Recommendations {
			switch {
			case len(r.Recommendations) == 1:
				_, _ = fmt.Fprintf(&sb, "`%s` is a recommended module.", r.Recommendations[i])
			case (i+1) != len(r.Recommendations) && (i+1) == (len(r.Recommendations)-1):
				_, _ = fmt.Fprintf(&sb, "`%s` ", r.Recommendations[i])
			case (i + 1) != len(r.Recommendations):
				_, _ = fmt.Fprintf(&sb, "`%s`, ", r.Recommendations[i])
			default:
				_, _ = fmt.Fprintf(&sb, "and `%s` are recommended modules.", r.Recommendations[i])
			}
		}
	}

	if r.Reason != "" {
		if sb.Len() > 0 {
			_, _ = fmt.Fprintf(&sb, " %s.", strings.TrimRight(r.Reason, "."))
		} else {
			_, _ = fmt.Fprintf(&sb, "%s.", strings.TrimRight(r.Reason, "."))
		}
	}

	return sb.String()
}

// IsCurrentModuleARecommendation returns true if the current module is in the Recommendations list.
func (r *BlockedModule) IsCurrentModuleARecommendation(currentModuleName string) bool {
	if r == nil {
		return false
	}

	for n := range r.Recommendations {
		if strings.TrimSpace(currentModuleName) == strings.TrimSpace(r.Recommendations[n]) {
			return true
		}
	}

	return false
}

// HasRecommendations returns true if the blocked package has recommended modules.
func (r *BlockedModule) HasRecommendations() bool {
	if r == nil {
		return false
	}

	return len(r.Recommendations) > 0
}
