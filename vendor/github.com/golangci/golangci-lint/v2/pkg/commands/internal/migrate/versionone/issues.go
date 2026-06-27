package versionone

type Issues struct {
	IncludeDefaultExcludes []string      `mapstructure:"include"`
	ExcludeCaseSensitive   *bool         `mapstructure:"exclude-case-sensitive"`
	ExcludePatterns        []string      `mapstructure:"exclude"`
	ExcludeRules           []ExcludeRule `mapstructure:"exclude-rules"`
	UseDefaultExcludes     *bool         `mapstructure:"exclude-use-default"`

	ExcludeGenerated *string `mapstructure:"exclude-generated"`

	ExcludeFiles []string `mapstructure:"exclude-files"`
	ExcludeDirs  []string `mapstructure:"exclude-dirs"`

	UseDefaultExcludeDirs *bool `mapstructure:"exclude-dirs-use-default"`

	MaxIssuesPerLinter *int  `mapstructure:"max-issues-per-linter"`
	MaxSameIssues      *int  `mapstructure:"max-same-issues"`
	UniqByLine         *bool `mapstructure:"uniq-by-line"`

	DiffFromRevision  *string `mapstructure:"new-from-rev"`
	DiffFromMergeBase *string `mapstructure:"new-from-merge-base"`
	DiffPatchFilePath *string `mapstructure:"new-from-patch"`
	WholeFiles        *bool   `mapstructure:"whole-files"`
	Diff              *bool   `mapstructure:"new"`

	NeedFix *bool `mapstructure:"fix"`
}

type ExcludeRule struct {
	BaseRule `mapstructure:",squash"`
}
