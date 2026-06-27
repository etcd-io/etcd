package versionone

type Severity struct {
	Default       *string        `mapstructure:"default-severity"`
	CaseSensitive *bool          `mapstructure:"case-sensitive"`
	Rules         []SeverityRule `mapstructure:"rules"`
}

type SeverityRule struct {
	BaseRule `mapstructure:",squash"`
	Severity *string `mapstructure:"severity"`
}
