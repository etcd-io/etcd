package gomodguard

import (
	"fmt"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Blocked is a list of modules that are
// blocked and not to be used.
type Blocked struct {
	Modules                BlockedModules  `yaml:"modules"`
	Versions               BlockedVersions `yaml:"versions"`
	LocalReplaceDirectives bool            `yaml:"local_replace_directives"`
}

// BlockedVersion has a version constraint a reason why the module version is blocked.
type BlockedVersion struct {
	Version string `yaml:"version"`
	Reason  string `yaml:"reason"`
}

// IsLintedModuleVersionBlocked returns true if a version constraint is specified and the
// linted module version matches the constraint.
func (r *BlockedVersion) IsLintedModuleVersionBlocked(lintedModuleVersion string) (bool, error) {
	if r.Version == "" {
		return false, nil
	}

	constraint, err := semver.NewConstraint(r.Version)
	if err != nil {
		return true, err
	}

	version, err := semver.NewVersion(lintedModuleVersion)
	if err != nil {
		return true, err
	}

	return constraint.Check(version), nil
}

// Message returns the reason why the module version is blocked.
func (r *BlockedVersion) Message(lintedModuleVersion string) string {
	var sb strings.Builder

	// Add version contraint to message.
	_, _ = fmt.Fprintf(&sb, "version `%s` is blocked because it does not meet the version constraint `%s`.",
		lintedModuleVersion, r.Version)

	if r.Reason == "" {
		return sb.String()
	}

	// Add reason to message.
	_, _ = fmt.Fprintf(&sb, " %s.", strings.TrimRight(r.Reason, "."))

	return sb.String()
}

// BlockedModule has alternative modules to use and a reason why the module is blocked.
type BlockedModule struct {
	Recommendations []string `yaml:"recommendations"`
	Reason          string   `yaml:"reason"`
}

// IsCurrentModuleARecommendation returns true if the current module is in the Recommendations list.
//
// If the current go.mod file being linted is a recommended module of a
// blocked module and it imports that blocked module, do not set as blocked.
// This could mean that the linted module is a wrapper for that blocked module.
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

// Message returns the reason why the module is blocked and a list of recommended modules if provided.
func (r *BlockedModule) Message() string {
	var sb strings.Builder

	// Add recommendations to message
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

	if r.Reason == "" {
		return sb.String()
	}

	// Add reason to message
	if sb.Len() == 0 {
		_, _ = fmt.Fprintf(&sb, "%s.", strings.TrimRight(r.Reason, "."))
	} else {
		_, _ = fmt.Fprintf(&sb, " %s.", strings.TrimRight(r.Reason, "."))
	}

	return sb.String()
}

// HasRecommendations returns true if the blocked package has
// recommended modules.
func (r *BlockedModule) HasRecommendations() bool {
	if r == nil {
		return false
	}

	return len(r.Recommendations) > 0
}

// BlockedVersions a list of blocked modules by a version constraint.
type BlockedVersions []map[string]BlockedVersion

// Get returns the module names that are blocked.
func (b BlockedVersions) Get() []string {
	modules := make([]string, len(b))

	for n := range b {
		for module := range b[n] {
			modules[n] = module
			break
		}
	}

	return modules
}

// GetBlockReason returns a block version if one is set for the provided linted module name.
func (b BlockedVersions) GetBlockReason(lintedModuleName string) *BlockedVersion {
	for _, blockedModule := range b {
		for blockedModuleName, blockedVersion := range blockedModule {
			if strings.TrimSpace(lintedModuleName) == strings.TrimSpace(blockedModuleName) {
				return &blockedVersion
			}
		}
	}

	return nil
}

// BlockedModules a list of blocked modules.
type BlockedModules []map[string]BlockedModule

// Get returns the module names that are blocked.
func (b BlockedModules) Get() []string {
	modules := make([]string, len(b))

	for n := range b {
		for module := range b[n] {
			modules[n] = module
			break
		}
	}

	return modules
}

// GetBlockReason returns a block module if one is set for the provided linted module name.
func (b BlockedModules) GetBlockReason(lintedModuleName string) *BlockedModule {
	for _, blockedModule := range b {
		for blockedModuleName, blockedModule := range blockedModule {
			if strings.TrimSpace(lintedModuleName) == strings.TrimSpace(blockedModuleName) {
				return &blockedModule
			}
		}
	}

	return nil
}
