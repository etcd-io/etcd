package model

// Rule represents a rule.
type Rule string

const (
	// PkgDocRule represents the "pkg-doc" rule.
	PkgDocRule Rule = "pkg-doc"
	// SinglePkgDocRule represents the "single-pkg-doc" rule.
	SinglePkgDocRule Rule = "single-pkg-doc"
	// RequirePkgDocRule represents the "require-pkg-doc" rule.
	RequirePkgDocRule Rule = "require-pkg-doc"
	// StartWithNameRule represents the "start-with-name" rule.
	StartWithNameRule Rule = "start-with-name"
	// RequireDocRule represents the "require-doc" rule.
	RequireDocRule Rule = "require-doc"
	// DeprecatedRule represents the "deprecated" rule.
	DeprecatedRule Rule = "deprecated"
	// RequireStdlibDoclinkRule represents the "require-stdlib-doclink" rule.
	RequireStdlibDoclinkRule Rule = "require-stdlib-doclink"
	// MaxLenRule represents the "max-len" rule.
	MaxLenRule Rule = "max-len"
	// NoUnusedLinkRule represents the "no-unused-link" rule.
	NoUnusedLinkRule Rule = "no-unused-link"
)

// AllRules is the set of all supported rules.
var AllRules = func() RuleSet {
	return RuleSet{}.Add(
		PkgDocRule,
		SinglePkgDocRule,
		RequirePkgDocRule,
		StartWithNameRule,
		RequireDocRule,
		DeprecatedRule,
		RequireStdlibDoclinkRule,
		MaxLenRule,
		NoUnusedLinkRule,
	)
}()
