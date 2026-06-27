package versionone

type BaseRule struct {
	Linters    []string `mapstructure:"linters"`
	Path       *string  `mapstructure:"path"`
	PathExcept *string  `mapstructure:"path-except"`
	Text       *string  `mapstructure:"text"`
	Source     *string  `mapstructure:"source"`
}
